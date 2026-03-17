package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/display"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

var (
	ErrRequestNotFound          = errors.New("request not found")
	ErrRequestNotPending        = errors.New("request is not in pending status")
	ErrDuplicateRequest         = errors.New("a pending request with the same identity already exists")
	ErrIdempotencyConflict      = errors.New("request with this idempotency key already exists")
	ErrSelfApproval             = errors.New("maker cannot approve their own request")
	ErrAlreadyActioned          = errors.New("checker has already acted on this request")
	ErrInvalidCheckerRole       = errors.New("checker does not have a required role")
	ErrInvalidCheckerPermission = errors.New("checker does not have a required permission")
	ErrNotEligibleReviewer      = errors.New("checker is not in the eligible reviewers list")
	ErrInvalidStage             = errors.New("request is at an invalid stage for this policy")
	ErrMissingIdentityFields    = errors.New("missing identity field in payload")
)

type RequestService struct {
	requests          store.RequestStore
	approvals         store.ApprovalStore
	policies          store.PolicyStore
	audits            store.AuditStore
	authorizationHook *auth.AuthorizationHook
	enqueueWebhooks   func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, req *model.Request, approvals []model.Approval) error
	signalWebhooks    func()
	runInTx           func(ctx context.Context, fn func(tx *store.Stores) error) error
	signalSSE         func(requestID uuid.UUID)
	metrics           *metrics.Metrics
}

// SetSSESignal configures the callback invoked after a request's state changes
// to notify connected SSE clients. Called post-commit alongside signalWebhooks.
func (s *RequestService) SetSSESignal(signal func(requestID uuid.UUID)) {
	s.signalSSE = signal
}

// SetMetrics sets the optional Prometheus metrics collector.
func (s *RequestService) SetMetrics(m *metrics.Metrics) {
	s.metrics = m
}

func NewRequestService(
	requests store.RequestStore,
	approvals store.ApprovalStore,
	policies store.PolicyStore,
	audits store.AuditStore,
	authorizationHook *auth.AuthorizationHook,
) *RequestService {
	return &RequestService{
		requests:          requests,
		approvals:         approvals,
		policies:          policies,
		audits:            audits,
		authorizationHook: authorizationHook,
	}
}

// SetWebhookDispatch configures transactional webhook dispatch via an outbox table.
// enqueue writes outbox rows (should be called inside a transaction), signal wakes
// the delivery worker (should be called after the transaction commits), and runInTx
// executes a function within a database transaction.
func (s *RequestService) SetWebhookDispatch(
	runInTx func(ctx context.Context, fn func(tx *store.Stores) error) error,
	enqueue func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, req *model.Request, approvals []model.Approval) error,
	signal func(),
) {
	s.runInTx = runInTx
	s.enqueueWebhooks = enqueue
	s.signalWebhooks = signal
}

func (s *RequestService) Create(ctx context.Context, req *model.Request) (*model.Request, error) {
	if req.TenantID == "" {
		req.TenantID = auth.TenantIDFromContext(ctx)
	}

	// Check idempotency key first
	if req.IdempotencyKey != nil {
		existing, err := s.requests.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("checking idempotency key: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	// Look up the policy
	policy, err := s.policies.GetByRequestType(ctx, req.Type)
	if err != nil {
		return nil, fmt.Errorf("looking up policy: %w", err)
	}
	if policy == nil {
		return nil, ErrPolicyNotFound
	}

	// Compute fingerprint if policy has identity fields
	if len(policy.IdentityFields) > 0 {
		fingerprint, err := computeFingerprint(req.Payload, policy.IdentityFields)
		if err != nil {
			return nil, fmt.Errorf("computing fingerprint: %w", err)
		}
		req.Fingerprint = &fingerprint

		// Check for duplicate pending request
		existing, err := s.requests.FindPendingByFingerprint(ctx, req.Type, fingerprint)
		if err != nil {
			return nil, fmt.Errorf("checking duplicate: %w", err)
		}
		if existing != nil {
			return nil, ErrDuplicateRequest
		}
	}

	// Set expiry from policy
	if policy.AutoExpireDuration != nil {
		expiresAt := time.Now().UTC().Add(*policy.AutoExpireDuration)
		req.ExpiresAt = &expiresAt
	}

	// Resolve display template if not already provided by the consumer
	resolveDisplayTemplate(req, policy)

	// Start at stage 0
	req.CurrentStage = 0

	if err := s.requests.Create(ctx, req); err != nil {
		return nil, err
	}

	// Audit log
	s.audit(ctx, req.ID, "created", req.MakerID, nil)

	if s.metrics != nil {
		s.metrics.RequestsTotal.WithLabelValues("created").Inc()
		s.metrics.PendingRequestsGauge.Inc()
	}

	return req, nil
}

func (s *RequestService) GetByID(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	req, err := s.requests.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}

	// Attach approvals
	approvals, err := s.approvals.ListByRequestID(ctx, id)
	if err != nil {
		return nil, err
	}
	req.Approvals = approvals

	return req, nil
}

func (s *RequestService) List(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
	return s.requests.List(ctx, filter)
}

func (s *RequestService) Approve(ctx context.Context, requestID uuid.UUID, checkerID string, roles []string, comment *string) (*model.Request, error) {
	return s.processDecision(ctx, requestID, checkerID, roles, model.DecisionApproved, comment)
}

func (s *RequestService) Reject(ctx context.Context, requestID uuid.UUID, checkerID string, roles []string, comment *string) (*model.Request, error) {
	return s.processDecision(ctx, requestID, checkerID, roles, model.DecisionRejected, comment)
}

func (s *RequestService) Cancel(ctx context.Context, requestID uuid.UUID, makerID string) (*model.Request, error) {
	req, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}
	if req.Status != model.StatusPending {
		return nil, ErrRequestNotPending
	}
	if req.MakerID != makerID {
		return nil, errors.New("only the maker can cancel their own request")
	}

	req.Status = model.StatusCancelled
	approvals, err := s.approvals.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("loading approvals for cancel callback: %w", err)
	}

	if s.runInTx != nil && s.enqueueWebhooks != nil {
		err := s.runInTx(ctx, func(txStores *store.Stores) error {
			if err := txStores.Requests.UpdateStatus(ctx, requestID, model.StatusCancelled); err != nil {
				return err
			}
			return s.enqueueWebhooks(ctx, txStores.Outbox, txStores.Webhooks, req, approvals)
		})
		if err != nil {
			if errors.Is(err, store.ErrStatusConflict) {
				return nil, ErrRequestNotPending
			}
			return nil, err
		}
		if s.signalWebhooks != nil {
			s.signalWebhooks()
		}
		if s.signalSSE != nil {
			s.signalSSE(requestID)
		}
	} else {
		if err := s.requests.UpdateStatus(ctx, requestID, model.StatusCancelled); err != nil {
			if errors.Is(err, store.ErrStatusConflict) {
				return nil, ErrRequestNotPending
			}
			return nil, err
		}
		if s.signalSSE != nil {
			s.signalSSE(requestID)
		}
	}

	s.audit(ctx, requestID, "cancelled", makerID, nil)

	if s.metrics != nil {
		s.metrics.RequestsTotal.WithLabelValues("cancelled").Inc()
		s.metrics.PendingRequestsGauge.Dec()
		s.metrics.RequestResolutionDuration.Observe(time.Since(req.CreatedAt).Seconds())
	}

	return req, nil
}

func (s *RequestService) processDecision(ctx context.Context, requestID uuid.UUID, checkerID string, roles []string, decision model.Decision, comment *string) (*model.Request, error) {
	req, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}
	if req.Status != model.StatusPending {
		return nil, ErrRequestNotPending
	}

	// Maker cannot approve their own request
	if req.MakerID == checkerID {
		return nil, ErrSelfApproval
	}

	// Look up policy for role validation and threshold logic
	policy, err := s.policies.GetByRequestType(ctx, req.Type)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, ErrPolicyNotFound
	}

	// Get the current stage definition
	stage := policy.StageAt(req.CurrentStage)
	if stage == nil {
		return nil, ErrInvalidStage
	}

	// Check if checker already acted on this stage
	already, err := s.approvals.ExistsByCheckerAndStage(ctx, requestID, checkerID, req.CurrentStage)
	if err != nil {
		return nil, err
	}
	if already {
		return nil, ErrAlreadyActioned
	}

	permissions := auth.PermissionsFromContext(ctx)
	if err := validateAuthorization(stage, roles, permissions); err != nil {
		return nil, err
	}

	// Check eligible reviewers (per-request whitelist)
	if len(req.EligibleReviewers) > 0 && !slices.Contains(req.EligibleReviewers, checkerID) {
		return nil, ErrNotEligibleReviewer
	}

	// Dynamic authorization hook (external check via consuming system)
	if policy.DynamicAuthorizationURL != nil && *policy.DynamicAuthorizationURL != "" && s.authorizationHook != nil {
		hookReq := model.AuthorizationHookRequest{
			RequestID:   requestID,
			RequestType: req.Type,
			CheckerID:   checkerID,
			MakerID:     req.MakerID,
			Payload:     req.Payload,
		}
		secret := ""
		if policy.DynamicAuthorizationSecret != nil {
			secret = *policy.DynamicAuthorizationSecret
		}
		if err := s.authorizationHook.Check(ctx, *policy.DynamicAuthorizationURL, secret, hookReq); err != nil {
			return nil, err
		}
	}

	approval := &model.Approval{
		RequestID:  requestID,
		CheckerID:  checkerID,
		Decision:   decision,
		StageIndex: req.CurrentStage,
		Comment:    comment,
	}

	var newStatus *model.RequestStatus
	var newStage *int

	if s.runInTx != nil {
		// Transactional path: lock the request row, insert vote, count inside
		// the transaction, and apply any resulting state change atomically.
		// The FOR UPDATE lock serializes concurrent decisions on the same
		// request, preventing missed stage advances and post-terminal votes.
		err := s.runInTx(ctx, func(txStores *store.Stores) error {
			// Lock the request row to serialize concurrent decision processing
			lockedReq, lockErr := txStores.Requests.GetByIDForUpdate(ctx, requestID)
			if lockErr != nil {
				return lockErr
			}
			if lockedReq == nil || lockedReq.Status != model.StatusPending {
				return store.ErrStatusConflict
			}
			// Use the locked version's stage — it may have advanced since our
			// initial read above.
			approval.StageIndex = lockedReq.CurrentStage

			if err := txStores.Approvals.Create(ctx, approval); err != nil {
				return fmt.Errorf("recording decision: %w", err)
			}

			// Count votes inside the transaction where we hold the lock
			approvalCount, err := txStores.Approvals.CountByDecisionAndStage(ctx, requestID, model.DecisionApproved, lockedReq.CurrentStage)
			if err != nil {
				return err
			}
			rejectionCount, err := txStores.Approvals.CountByDecisionAndStage(ctx, requestID, model.DecisionRejected, lockedReq.CurrentStage)
			if err != nil {
				return err
			}

			newStatus, newStage = evaluateWithCounts(approvalCount, rejectionCount, lockedReq.CurrentStage, policy)

			if newStatus != nil {
				// Terminal status (approved or rejected)
				if err := txStores.Requests.UpdateStatus(ctx, requestID, *newStatus); err != nil {
					return err
				}
				if s.enqueueWebhooks != nil {
					resolveApprovals, err := txStores.Approvals.ListByRequestID(ctx, requestID)
					if err != nil {
						return fmt.Errorf("loading approvals for callback: %w", err)
					}
					req.Status = *newStatus
					req.Approvals = resolveApprovals
					return s.enqueueWebhooks(ctx, txStores.Outbox, txStores.Webhooks, req, resolveApprovals)
				}
			} else if newStage != nil {
				// Stage advance — still pending but move to next stage
				if err := txStores.Requests.UpdateStageAndStatus(ctx, requestID, *newStage, model.StatusPending); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, store.ErrStatusConflict) {
				return nil, ErrRequestNotPending
			}
			if errors.Is(err, store.ErrDuplicateApproval) {
				return nil, ErrAlreadyActioned
			}
			return nil, err
		}

		// Post-commit side effects
		s.audit(ctx, requestID, string(decision), checkerID, nil)

		if newStatus != nil {
			if s.signalWebhooks != nil {
				s.signalWebhooks()
			}
			if s.signalSSE != nil {
				s.signalSSE(requestID)
			}
			if s.metrics != nil {
				s.metrics.RequestsTotal.WithLabelValues(string(*newStatus)).Inc()
				s.metrics.PendingRequestsGauge.Dec()
				s.metrics.RequestResolutionDuration.Observe(time.Since(req.CreatedAt).Seconds())
			}
		} else if newStage != nil {
			fromStage := req.CurrentStage
			req.CurrentStage = *newStage
			stageDetails, _ := json.Marshal(map[string]int{"from_stage": fromStage, "to_stage": *newStage})
			s.audit(ctx, requestID, "stage_advanced", "system", stageDetails)
			if s.signalSSE != nil {
				s.signalSSE(requestID)
			}
		}
	} else {
		// Fallback: no transaction support. Pre-read counts and predict outcome.
		// This path is inherently racy under concurrency but provides a working
		// single-instance fallback.
		approvalCount, err := s.approvals.CountByDecisionAndStage(ctx, requestID, model.DecisionApproved, req.CurrentStage)
		if err != nil {
			return nil, err
		}
		rejectionCount, err := s.approvals.CountByDecisionAndStage(ctx, requestID, model.DecisionRejected, req.CurrentStage)
		if err != nil {
			return nil, err
		}

		predictedApprovals := approvalCount
		predictedRejections := rejectionCount
		if decision == model.DecisionApproved {
			predictedApprovals++
		} else {
			predictedRejections++
		}

		newStatus, newStage = evaluateWithCounts(predictedApprovals, predictedRejections, req.CurrentStage, policy)

		if err := s.approvals.Create(ctx, approval); err != nil {
			if errors.Is(err, store.ErrDuplicateApproval) {
				return nil, ErrAlreadyActioned
			}
			return nil, fmt.Errorf("recording decision: %w", err)
		}

		s.audit(ctx, requestID, string(decision), checkerID, nil)

		if newStatus != nil && *newStatus != req.Status {
			req.Status = *newStatus
			if err := s.requests.UpdateStatus(ctx, requestID, *newStatus); err != nil {
				if errors.Is(err, store.ErrStatusConflict) {
					return nil, ErrRequestNotPending
				}
				return nil, err
			}
			if newStatus.IsTerminal() && s.metrics != nil {
				s.metrics.RequestsTotal.WithLabelValues(string(*newStatus)).Inc()
				s.metrics.PendingRequestsGauge.Dec()
				s.metrics.RequestResolutionDuration.Observe(time.Since(req.CreatedAt).Seconds())
			}
		} else if newStage != nil && *newStage != req.CurrentStage {
			fromStage := req.CurrentStage
			if err := s.requests.UpdateStageAndStatus(ctx, requestID, *newStage, model.StatusPending); err != nil {
				if errors.Is(err, store.ErrStatusConflict) {
					return nil, ErrRequestNotPending
				}
				return nil, err
			}
			req.CurrentStage = *newStage
			stageDetails, _ := json.Marshal(map[string]int{"from_stage": fromStage, "to_stage": *newStage})
			s.audit(ctx, requestID, "stage_advanced", "system", stageDetails)
		}
	}

	// Refresh approvals
	allApprovals, err := s.approvals.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	req.Approvals = allApprovals

	return req, nil
}

// evaluateWithCounts determines the outcome based on approval/rejection counts.
// This is a pure function with no database access — counts are read beforehand
// so the caller can choose the correct transaction boundary.
//
// Returns:
//   - (*RequestStatus, nil) — terminal status change (approved/rejected)
//   - (nil, *int) — stage advance (next stage index)
//   - (nil, nil) — no change yet
func evaluateWithCounts(approvalCount, rejectionCount int, currentStage int, policy *model.Policy) (*model.RequestStatus, *int) {
	stage := policy.StageAt(currentStage)
	if stage == nil {
		return nil, nil
	}

	// Check rejection policy for the current stage
	switch stage.RejectionPolicy {
	case model.RejectionPolicyAny:
		if rejectionCount > 0 {
			status := model.StatusRejected
			return &status, nil
		}
	case model.RejectionPolicyThreshold:
		if stage.MaxCheckers != nil {
			remaining := *stage.MaxCheckers - approvalCount - rejectionCount
			if approvalCount+remaining < stage.RequiredApprovals {
				status := model.StatusRejected
				return &status, nil
			}
		}
	}

	// Check if we have enough approvals for this stage
	if approvalCount >= stage.RequiredApprovals {
		nextStage := currentStage + 1
		if nextStage >= policy.TotalStages() {
			// All stages complete — request approved
			status := model.StatusApproved
			return &status, nil
		}
		// Advance to next stage
		return nil, &nextStage
	}

	// Not enough votes either way yet
	return nil, nil
}

func (s *RequestService) audit(ctx context.Context, requestID uuid.UUID, action string, actorID string, details json.RawMessage) {
	log := &model.AuditLog{
		TenantID:  auth.TenantIDFromContext(ctx),
		RequestID: requestID,
		Action:    action,
		ActorID:   actorID,
		Details:   details,
	}
	if err := s.audits.Create(ctx, log); err != nil {
		slog.Error("failed to create audit log", "error", err, "request_id", requestID, "action", action)
	}
}

func computeFingerprint(payload json.RawMessage, identityFields []string) (string, error) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", fmt.Errorf("unmarshaling payload: %w", err)
	}

	// Extract identity field values in deterministic order
	sort.Strings(identityFields)
	values := make(map[string]any)
	for _, field := range identityFields {
		val, ok := data[field]
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrMissingIdentityFields, field)
		}
		values[field] = val
	}

	// Serialize and hash
	canonical, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshaling identity values: %w", err)
	}

	hash := sha256.Sum256(canonical)
	return fmt.Sprintf("%x", hash), nil
}

func validateCheckerRoles(stage *model.ApprovalStage, checkerRoles []string) error {
	if stage.AllowedCheckerRoles == nil {
		return nil
	}

	var allowedRoles []string
	if err := json.Unmarshal(stage.AllowedCheckerRoles, &allowedRoles); err != nil {
		slog.Error("failed to parse allowed_checker_roles", "error", err, "stage_index", stage.Index)
		return fmt.Errorf("invalid allowed_checker_roles configuration: %w", err)
	}

	if len(allowedRoles) == 0 {
		return nil
	}

	roleSet := make(map[string]bool)
	for _, r := range checkerRoles {
		roleSet[r] = true
	}

	for _, allowed := range allowedRoles {
		if roleSet[allowed] {
			return nil
		}
	}

	return ErrInvalidCheckerRole
}

func validateCheckerPermissions(stage *model.ApprovalStage, checkerPermissions []string) error {
	if stage.AllowedPermissions == nil {
		return nil
	}

	var allowedPermissions []string
	if err := json.Unmarshal(stage.AllowedPermissions, &allowedPermissions); err != nil {
		slog.Error("failed to parse allowed_permissions", "error", err, "stage_index", stage.Index)
		return fmt.Errorf("invalid allowed_permissions configuration: %w", err)
	}

	if len(allowedPermissions) == 0 {
		return nil
	}

	permSet := make(map[string]bool)
	for _, p := range checkerPermissions {
		permSet[p] = true
	}

	for _, allowed := range allowedPermissions {
		if permSet[allowed] {
			return nil
		}
	}

	return ErrInvalidCheckerPermission
}

// validateAuthorization combines role and permission checks based on the
// stage's authorization_mode. When only one of roles/permissions is configured,
// the mode is implicit. When both are configured, the mode must be set.
func validateAuthorization(stage *model.ApprovalStage, checkerRoles, checkerPermissions []string) error {
	hasRoles := stage.AllowedCheckerRoles != nil
	hasPermissions := stage.AllowedPermissions != nil

	if !hasRoles && !hasPermissions {
		return nil
	}

	if hasRoles && !hasPermissions {
		return validateCheckerRoles(stage, checkerRoles)
	}

	if !hasRoles && hasPermissions {
		return validateCheckerPermissions(stage, checkerPermissions)
	}

	// Both are set — use authorization_mode
	switch stage.AuthorizationMode {
	case model.AuthModeAny:
		roleErr := validateCheckerRoles(stage, checkerRoles)
		permErr := validateCheckerPermissions(stage, checkerPermissions)
		if roleErr == nil || permErr == nil {
			return nil
		}
		return roleErr
	case model.AuthModeAll:
		if err := validateCheckerRoles(stage, checkerRoles); err != nil {
			return err
		}
		return validateCheckerPermissions(stage, checkerPermissions)
	default:
		return validateCheckerRoles(stage, checkerRoles)
	}
}

// resolveDisplayTemplate resolves the policy's display template against the request
// payload and merges the result into metadata.display. If the consumer already
// provided metadata.display (override), or the policy has no template, this is a no-op.
func resolveDisplayTemplate(req *model.Request, policy *model.Policy) {
	if len(policy.DisplayTemplate) == 0 {
		return
	}

	// Check if consumer already provided metadata.display (override)
	if len(req.Metadata) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(req.Metadata, &meta); err == nil {
			if _, ok := meta["display"]; ok {
				return // consumer override — keep it
			}
		}
	}

	resolved, err := display.Resolve(policy.DisplayTemplate, req.Payload)
	if err != nil {
		slog.Warn("display template resolution failed", "error", err, "policy_id", policy.ID, "request_type", req.Type)
		return // degrade gracefully — don't block request creation
	}
	if resolved == nil {
		return
	}

	// Merge resolved display into metadata
	var meta map[string]json.RawMessage
	if len(req.Metadata) > 0 {
		if err := json.Unmarshal(req.Metadata, &meta); err != nil {
			meta = make(map[string]json.RawMessage)
		}
	} else {
		meta = make(map[string]json.RawMessage)
	}

	meta["display"] = resolved

	merged, err := json.Marshal(meta)
	if err != nil {
		return
	}
	req.Metadata = merged
}

// CanViewerAct determines whether the given viewer can approve or reject
// the request based on the same checks enforced by processDecision.
// The dynamic_authorization_url is intentionally skipped (HTTP call not
// suitable for the read path); it is still enforced on actual approve/reject.
func (s *RequestService) CanViewerAct(ctx context.Context, req *model.Request, viewerID string, viewerRoles, viewerPermissions []string) bool {
	if viewerID == "" {
		return false
	}
	if req.Status != model.StatusPending {
		return false
	}
	if req.MakerID == viewerID {
		return false
	}

	if len(req.EligibleReviewers) > 0 && !slices.Contains(req.EligibleReviewers, viewerID) {
		return false
	}

	for _, a := range req.Approvals {
		if a.CheckerID == viewerID && a.StageIndex == req.CurrentStage {
			return false
		}
	}

	policy, err := s.policies.GetByRequestType(ctx, req.Type)
	if err != nil || policy == nil {
		return false
	}

	stage := policy.StageAt(req.CurrentStage)
	if stage == nil {
		return false
	}

	if err := validateAuthorization(stage, viewerRoles, viewerPermissions); err != nil {
		return false
	}

	return true
}
