// Package main implements a sample banking wire transfer application that
// integrates with the Quorum maker-checker system. Wire transfers require
// multi-stage approval before execution.
//
// On startup the app registers a "wire_transfer" policy with Quorum (idempotent).
// Users create transfers through the web UI, which are submitted to Quorum for
// approval. When Quorum reaches a terminal decision it calls back via webhook,
// and the app updates the transfer status accordingly.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

type config struct {
	QuorumAPIURL  string // Base URL of the Quorum server
	SelfURL       string // Publicly reachable URL of this app
	WebhookSecret string // Shared secret for HMAC-SHA256 webhook verification
	Port          string // HTTP listen port
}

func loadConfig() config {
	return config{
		QuorumAPIURL:  envOr("QUORUM_API_URL", "http://localhost:8080"),
		SelfURL:       envOr("SELF_URL", "http://localhost:3001"),
		WebhookSecret: envOr("WEBHOOK_SECRET", "banking-webhook-secret"),
		Port:          envOr("PORT", "3001"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Transfer model
// ---------------------------------------------------------------------------

// TransferStatus represents the lifecycle state of a wire transfer.
type TransferStatus string

const (
	StatusPending  TransferStatus = "pending"
	StatusApproved TransferStatus = "approved"
	StatusRejected TransferStatus = "rejected"
	StatusExecuted TransferStatus = "executed"
)

// Transfer represents a wire transfer request tracked by this application.
type Transfer struct {
	ID              string         `json:"id"`
	FromUser        string         `json:"from_user"`
	SourceAccount   string         `json:"source_account"`
	Amount          string         `json:"amount"`
	Destination     string         `json:"destination"`
	Status          TransferStatus `json:"status"`
	QuorumRequestID string         `json:"quorum_request_id"`
	CreatedAt       time.Time      `json:"created_at"`
}

// ---------------------------------------------------------------------------
// In-memory transfer store
// ---------------------------------------------------------------------------

type transferStore struct {
	mu        sync.Mutex
	transfers map[string]*Transfer
	order     []string // insertion order for listing
	nextID    int
}

func newTransferStore() *transferStore {
	return &transferStore{
		transfers: make(map[string]*Transfer),
	}
}

func (s *transferStore) Create(t *Transfer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	t.ID = fmt.Sprintf("TXN-%04d", s.nextID)
	t.CreatedAt = time.Now()
	t.Status = StatusPending
	s.transfers[t.ID] = t
	s.order = append(s.order, t.ID)
}

func (s *transferStore) Get(id string) (*Transfer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.transfers[id]
	return t, ok
}

// FindByQuorumID looks up a transfer by its Quorum request UUID.
func (s *transferStore) FindByQuorumID(quorumID string) (*Transfer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.transfers {
		if t.QuorumRequestID == quorumID {
			return t, true
		}
	}
	return nil, false
}

func (s *transferStore) SetStatus(id string, status TransferStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.transfers[id]; ok {
		t.Status = status
	}
}

// List returns all transfers in reverse-chronological order.
func (s *transferStore) List() []*Transfer {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Transfer, len(s.order))
	for i, id := range s.order {
		result[len(s.order)-1-i] = s.transfers[id]
	}
	return result
}

// ---------------------------------------------------------------------------
// Quorum API types
// ---------------------------------------------------------------------------

// quorumCreateRequest is the body sent to POST /api/v1/requests.
type quorumCreateRequest struct {
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	CallbackURL *string         `json:"callback_url,omitempty"`
}

// quorumCreateResponse is the subset of fields we need from the response.
type quorumCreateResponse struct {
	ID string `json:"id"`
}

// quorumPolicyRequest is the body sent to POST /api/v1/policies.
type quorumPolicyRequest struct {
	Name            string                `json:"name"`
	RequestType     string                `json:"request_type"`
	Stages          []quorumApprovalStage `json:"stages"`
	DisplayTemplate json.RawMessage       `json:"display_template,omitempty"`
}

type quorumApprovalStage struct {
	Index             int    `json:"index"`
	Name              string `json:"name"`
	RequiredApprovals int    `json:"required_approvals"`
	RejectionPolicy   string `json:"rejection_policy"`
}

// webhookPayload matches the Quorum WebhookPayload model.
type webhookPayload struct {
	Event   string          `json:"event"`
	Request json.RawMessage `json:"request"`
}

// webhookRequest is a minimal parse of the nested request object.
type webhookRequest struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------
// Application
// ---------------------------------------------------------------------------

type app struct {
	cfg    config
	store  *transferStore
	pages  map[string]*template.Template
	client *http.Client
}

// parseTemplates builds a per-page template set. Each page template is paired
// with the shared layout so that {{define "content"}} blocks do not collide.
func parseTemplates() map[string]*template.Template {
	pages := map[string]*template.Template{}
	pageFiles := []string{"dashboard.html", "new_transfer.html", "detail.html"}
	for _, page := range pageFiles {
		t := template.Must(template.ParseFiles(
			"templates/layout.html",
			"templates/"+page,
		))
		pages[page] = t
	}
	return pages
}

func main() {
	cfg := loadConfig()

	a := &app{
		cfg:    cfg,
		store:  newTransferStore(),
		pages:  parseTemplates(),
		client: &http.Client{Timeout: 10 * time.Second},
	}

	// Register the wire_transfer policy with Quorum on startup.
	a.registerPolicy()

	// Set up HTTP routes using Go 1.22+ method-based patterns.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("GET /transfer/new", a.handleNewTransferForm)
	mux.HandleFunc("POST /transfer/new", a.handleCreateTransfer)
	mux.HandleFunc("GET /transfer/{id}", a.handleTransferDetail)
	mux.HandleFunc("POST /webhooks/quorum", a.handleWebhook)

	log.Printf("Banking app listening on :%s", cfg.Port)
	log.Printf("Quorum API: %s", cfg.QuorumAPIURL)
	log.Printf("Webhook callback: %s/webhooks/quorum", cfg.SelfURL)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Policy registration
// ---------------------------------------------------------------------------

// registerPolicy creates the wire_transfer policy in Quorum.
// If the policy already exists (409 Conflict) it is silently ignored.
func (a *app) registerPolicy() {
	displayTemplate, _ := json.Marshal(map[string]any{
		"title":       "Wire Transfer - {{.source_account}}",
		"description": "Transfer of ${{.amount}} to {{.destination}}",
		"fields": []map[string]string{
			{"key": "source_account", "label": "Source Account"},
			{"key": "amount", "label": "Amount (USD)"},
			{"key": "destination", "label": "Destination"},
		},
	})

	policy := quorumPolicyRequest{
		Name:        "Wire Transfer Approval",
		RequestType: "wire_transfer",
		Stages: []quorumApprovalStage{
			{
				Index:             0,
				Name:              "Manager Review",
				RequiredApprovals: 1,
				RejectionPolicy:   "any",
			},
			{
				Index:             1,
				Name:              "Compliance Check",
				RequiredApprovals: 1,
				RejectionPolicy:   "any",
			},
		},
		DisplayTemplate: displayTemplate,
	}

	body, _ := json.Marshal(policy)
	req, err := http.NewRequest(http.MethodPost, a.cfg.QuorumAPIURL+"/api/v1/policies", bytes.NewReader(body))
	if err != nil {
		log.Printf("WARN: failed to build policy request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "banking-system")
	req.Header.Set("X-Tenant-ID", "banking")

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("WARN: failed to register policy with Quorum (will retry on next restart): %v", err)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		log.Println("Registered wire_transfer policy with Quorum")
	case http.StatusConflict:
		log.Println("wire_transfer policy already exists in Quorum (OK)")
	default:
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("WARN: unexpected status %d registering policy: %s", resp.StatusCode, string(respBody))
	}
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

// handleDashboard renders the list of all transfers.
func (a *app) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// The "GET /" pattern in Go 1.22+ matches all unmatched paths too,
	// so we restrict to the exact root.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"Transfers": a.store.List(),
		"QuorumURL": a.cfg.QuorumAPIURL,
	}
	a.render(w, "dashboard.html", data)
}

// handleNewTransferForm renders the wire transfer creation form.
func (a *app) handleNewTransferForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, "new_transfer.html", nil)
}

// handleCreateTransfer processes the form submission, saves the transfer
// locally, and submits it to Quorum for approval.
func (a *app) handleCreateTransfer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	fromUser := strings.TrimSpace(r.FormValue("from_user"))
	sourceAccount := strings.TrimSpace(r.FormValue("source_account"))
	amount := strings.TrimSpace(r.FormValue("amount"))
	destination := strings.TrimSpace(r.FormValue("destination"))

	if fromUser == "" || sourceAccount == "" || amount == "" || destination == "" {
		http.Error(w, "all fields are required", http.StatusBadRequest)
		return
	}

	// Create the transfer locally.
	transfer := &Transfer{
		FromUser:      fromUser,
		SourceAccount: sourceAccount,
		Amount:        amount,
		Destination:   destination,
	}
	a.store.Create(transfer)

	// Build the Quorum request payload.
	payload, _ := json.Marshal(map[string]string{
		"transfer_id":    transfer.ID,
		"from_user":      fromUser,
		"source_account": sourceAccount,
		"amount":         amount,
		"destination":    destination,
	})

	callbackURL := a.cfg.SelfURL + "/webhooks/quorum"
	quorumReq := quorumCreateRequest{
		Type:        "wire_transfer",
		Payload:     payload,
		CallbackURL: &callbackURL,
	}

	body, _ := json.Marshal(quorumReq)
	httpReq, err := http.NewRequest(http.MethodPost, a.cfg.QuorumAPIURL+"/api/v1/requests", bytes.NewReader(body))
	if err != nil {
		log.Printf("ERROR: failed to build Quorum request: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-User-ID", fromUser)
	httpReq.Header.Set("X-Tenant-ID", "banking")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		log.Printf("ERROR: failed to submit to Quorum: %v", err)
		// Transfer is created locally but not linked to Quorum.
		http.Redirect(w, r, "/transfer/"+transfer.ID, http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		var qResp quorumCreateResponse
		if err := json.NewDecoder(resp.Body).Decode(&qResp); err == nil {
			transfer.QuorumRequestID = qResp.ID
			log.Printf("Transfer %s submitted to Quorum as request %s", transfer.ID, qResp.ID)
		}
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("WARN: Quorum returned status %d: %s", resp.StatusCode, string(respBody))
	}

	http.Redirect(w, r, "/transfer/"+transfer.ID, http.StatusSeeOther)
}

// handleTransferDetail renders the detail page for a single transfer,
// including the embedded Quorum approval widget.
func (a *app) handleTransferDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	transfer, ok := a.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	data := map[string]any{
		"Transfer":  transfer,
		"QuorumURL": a.cfg.QuorumAPIURL,
	}
	a.render(w, "detail.html", data)
}

// ---------------------------------------------------------------------------
// Webhook handler
// ---------------------------------------------------------------------------

// handleWebhook receives callbacks from Quorum when a request reaches a
// terminal state (approved or rejected). It verifies the HMAC-SHA256 signature
// and updates the local transfer status.
func (a *app) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify HMAC-SHA256 signature.
	sigHeader := r.Header.Get("X-Signature-256")
	if !verifySignature(body, sigHeader, a.cfg.WebhookSecret) {
		log.Println("WARN: webhook signature verification failed")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse the webhook payload.
	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("WARN: failed to parse webhook payload: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Extract the request ID from the nested request object.
	var reqObj webhookRequest
	if err := json.Unmarshal(payload.Request, &reqObj); err != nil {
		log.Printf("WARN: failed to parse webhook request object: %v", err)
		http.Error(w, "invalid request object", http.StatusBadRequest)
		return
	}

	// Find the local transfer by Quorum request ID.
	transfer, ok := a.store.FindByQuorumID(reqObj.ID)
	if !ok {
		log.Printf("WARN: received webhook for unknown Quorum request %s", reqObj.ID)
		// Return 200 so Quorum does not retry for unknown requests.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Update the transfer status based on the event.
	switch payload.Event {
	case "approved":
		a.store.SetStatus(transfer.ID, StatusApproved)
		// In a real system, this is where you would execute the wire transfer
		// (e.g. call a payment gateway). For this demo we immediately mark
		// it as executed.
		a.store.SetStatus(transfer.ID, StatusExecuted)
		log.Printf("Transfer %s approved and executed (Quorum request %s)", transfer.ID, reqObj.ID)
	case "rejected":
		a.store.SetStatus(transfer.ID, StatusRejected)
		log.Printf("Transfer %s rejected (Quorum request %s)", transfer.ID, reqObj.ID)
	default:
		log.Printf("INFO: ignoring webhook event %q for request %s", payload.Event, reqObj.ID)
	}

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// HMAC verification
// ---------------------------------------------------------------------------

// verifySignature checks the HMAC-SHA256 signature of the webhook body.
// The expected format is "sha256=<hex-encoded-hmac>".
func verifySignature(body []byte, sigHeader, secret string) bool {
	if !strings.HasPrefix(sigHeader, "sha256=") {
		return false
	}

	receivedSig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := mac.Sum(nil)

	return hmac.Equal(receivedSig, expectedSig)
}

// ---------------------------------------------------------------------------
// Template rendering
// ---------------------------------------------------------------------------

// render executes the named page template (which includes the layout).
func (a *app) render(w http.ResponseWriter, name string, data any) {
	t, ok := a.pages[name]
	if !ok {
		log.Printf("ERROR: template %q not found", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("ERROR: template rendering failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
