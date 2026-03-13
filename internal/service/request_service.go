package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

var (
	ErrRequestNotFound       = errors.New("request not found")
	ErrRequestNotPending     = errors.New("request is not in pending status")
	ErrDuplicateRequest      = errors.New("a pending request with the same identity already exists")
	ErrIdempotencyConflict   = errors.New("request with this idempotency key already exists")
	ErrSelfApproval          = errors.New("maker cannot approve their own request")
	ErrAlreadyActioned       = errors.New("checker has already acted on this request")
	ErrInvalidCheckerRole    = errors.New("checker does not have a required role")
	ErrNotEligibleReviewer   = errors.New("checker is not in the eligible reviewers list")
	ErrInvalidStage          = errors.New("request is at an invalid stage for this policy")
	ErrMissingIdentityFields = errors.New("missing identity field in payload")
)

type RequestService struct {
	requests          store.RequestStore
	approvals         store.ApprovalStore
	policies          store.PolicyStore
	audits            store.AuditStore
	permissionChecker *auth.PermissionChecker
	onResolve         func(ctx context.Context, req *model.Request, approvals []model.Approval)
	metrics           *metrics.Metrics
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
	permissionChecker *auth.PermissionChecker,
) *RequestService {
	return &RequestService{
		requests:          requests,
		approvals:         approvals,
		policies:          policies,
		audits:            audits,
		permissionChecker: permissionChecker,
	}
}

// SetOnResolve sets a callback that fires when a request transitions to a terminal state.
func (s *RequestService) SetOnResolve(fn func(ctx context.Context, req *model.Request, approvals []model.Approval)) {
	s.onResolve = fn
}

func (s *RequestService) Create(ctx context.Context, req *model.Request) (*model.Request, error) {
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

	if err := s.requests.UpdateStatus(ctx, requestID, model.StatusCancelled); err != nil {
		return nil, err
	}
	req.Status = model.StatusCancelled

	s.audit(ctx, requestID, "cancelled", makerID, nil)

	if s.metrics != nil {
		s.metrics.RequestsTotal.WithLabelValues("cancelled").Inc()
		s.metrics.PendingRequestsGauge.Dec()
		s.metrics.RequestResolutionDuration.Observe(time.Since(req.CreatedAt).Seconds())
	}

	if s.onResolve != nil {
		approvals, _ := s.approvals.ListByRequestID(ctx, requestID)
		s.onResolve(ctx, req, approvals)
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

	// Validate checker roles against the current stage's allowed roles
	if err := validateCheckerRoles(stage, roles); err != nil {
		return nil, err
	}

	// Check eligible reviewers (per-request whitelist)
	if len(req.EligibleReviewers) > 0 {
		eligible := false
		for _, r := range req.EligibleReviewers {
			if r == checkerID {
				eligible = true
				break
			}
		}
		if !eligible {
			return nil, ErrNotEligibleReviewer
		}
	}

	// Permission check callback (dynamic check via consuming system)
	if policy.PermissionCheckURL != nil && *policy.PermissionCheckURL != "" && s.permissionChecker != nil {
		checkReq := model.PermissionCheckRequest{
			RequestID:    requestID,
			RequestType:  req.Type,
			CheckerID:    checkerID,
			CheckerRoles: roles,
			MakerID:      req.MakerID,
			Payload:      req.Payload,
		}
		if err := s.permissionChecker.Check(ctx, *policy.PermissionCheckURL, checkReq); err != nil {
			return nil, err
		}
	}

	// Record the approval/rejection with the current stage index
	approval := &model.Approval{
		RequestID:  requestID,
		CheckerID:  checkerID,
		Decision:   decision,
		StageIndex: req.CurrentStage,
		Comment:    comment,
	}
	if err := s.approvals.Create(ctx, approval); err != nil {
		return nil, fmt.Errorf("recording decision: %w", err)
	}

	s.audit(ctx, requestID, string(decision), checkerID, nil)

	// Evaluate state transition
	newStatus, newStage, err := s.evaluateStatus(ctx, requestID, req.CurrentStage, policy)
	if err != nil {
		return nil, err
	}

	if newStatus != nil && *newStatus != req.Status {
		// Terminal status change (approved or rejected)
		if err := s.requests.UpdateStatus(ctx, requestID, *newStatus); err != nil {
			return nil, err
		}
		req.Status = *newStatus

		if newStatus.IsTerminal() {
			if s.metrics != nil {
				s.metrics.RequestsTotal.WithLabelValues(string(*newStatus)).Inc()
				s.metrics.PendingRequestsGauge.Dec()
				s.metrics.RequestResolutionDuration.Observe(time.Since(req.CreatedAt).Seconds())
			}

			if s.onResolve != nil {
				approvals, _ := s.approvals.ListByRequestID(ctx, requestID)
				req.Approvals = approvals
				s.onResolve(ctx, req, approvals)
			}
		}
	} else if newStage != nil && *newStage != req.CurrentStage {
		// Stage advance — still pending but move to next stage
		if err := s.requests.UpdateStageAndStatus(ctx, requestID, *newStage, model.StatusPending); err != nil {
			return nil, err
		}
		req.CurrentStage = *newStage

		stageDetails, _ := json.Marshal(map[string]int{"from_stage": req.CurrentStage - 1, "to_stage": *newStage})
		s.audit(ctx, requestID, "stage_advanced", "system", stageDetails)
	}

	// Refresh approvals
	allApprovals, err := s.approvals.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	req.Approvals = allApprovals

	return req, nil
}

// evaluateStatus evaluates the current stage's approval/rejection state.
// Returns:
//   - (*RequestStatus, nil, nil) — terminal status change (approved/rejected)
//   - (nil, *int, nil) — stage advance (next stage index)
//   - (nil, nil, nil) — no change yet
func (s *RequestService) evaluateStatus(ctx context.Context, requestID uuid.UUID, currentStage int, policy *model.Policy) (*model.RequestStatus, *int, error) {
	stage := policy.StageAt(currentStage)
	if stage == nil {
		return nil, nil, ErrInvalidStage
	}

	approvalCount, err := s.approvals.CountByDecisionAndStage(ctx, requestID, model.DecisionApproved, currentStage)
	if err != nil {
		return nil, nil, err
	}
	rejectionCount, err := s.approvals.CountByDecisionAndStage(ctx, requestID, model.DecisionRejected, currentStage)
	if err != nil {
		return nil, nil, err
	}

	// Check rejection policy for the current stage
	switch stage.RejectionPolicy {
	case model.RejectionPolicyAny:
		if rejectionCount > 0 {
			status := model.StatusRejected
			return &status, nil, nil
		}
	case model.RejectionPolicyThreshold:
		if stage.MaxCheckers != nil {
			remaining := *stage.MaxCheckers - approvalCount - rejectionCount
			if approvalCount+remaining < stage.RequiredApprovals {
				status := model.StatusRejected
				return &status, nil, nil
			}
		}
	}

	// Check if we have enough approvals for this stage
	if approvalCount >= stage.RequiredApprovals {
		nextStage := currentStage + 1
		if nextStage >= policy.TotalStages() {
			// All stages complete — request approved
			status := model.StatusApproved
			return &status, nil, nil
		}
		// Advance to next stage
		return nil, &nextStage, nil
	}

	// Not enough votes either way yet
	return nil, nil, nil
}

func (s *RequestService) audit(ctx context.Context, requestID uuid.UUID, action string, actorID string, details json.RawMessage) {
	log := &model.AuditLog{
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
		return nil // If we can't parse, skip validation
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
