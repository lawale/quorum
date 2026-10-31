// Package main implements a sample banking wire transfer application that
// integrates with the Quorum maker-checker system through the Quorum Go
// client. Wire transfers require multi-stage approval before execution.
//
// The "wire_transfer" policy is registered by scripts/setup.sh. Users create
// transfers through the web UI, which are submitted to Quorum for approval.
// When Quorum reaches a terminal decision it calls back via webhook, and the
// app updates the transfer status accordingly.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lawale/quorum/clients/go"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

type config struct {
	QuorumAPIURL    string
	QuorumPublicURL string // browser-facing URL for the approval widget
	SelfURL         string
	WebhookSecret   string
	Port            string
	DatabaseURL     string
	BasePath        string // path prefix the app is served under behind a proxy, e.g. /demo/banking
}

func loadConfig() config {
	apiURL := envOr("QUORUM_API_URL", "http://localhost:8080")
	return config{
		QuorumAPIURL:    apiURL,
		QuorumPublicURL: envOr("QUORUM_PUBLIC_URL", apiURL),
		SelfURL:         envOr("SELF_URL", "http://localhost:3001"),
		WebhookSecret:   envOr("WEBHOOK_SECRET", "banking-webhook-secret"),
		Port:            envOr("PORT", "3001"),
		BasePath:        strings.TrimSuffix(os.Getenv("BASE_PATH"), "/"),
		DatabaseURL:     envOr("DATABASE_URL", "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable"),
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

type TransferStatus string

const (
	StatusPending  TransferStatus = "pending"
	StatusApproved TransferStatus = "approved"
	StatusRejected TransferStatus = "rejected"
	StatusExecuted TransferStatus = "executed"
)

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
// PostgreSQL transfer store
// ---------------------------------------------------------------------------

type transferStore struct {
	pool *pgxpool.Pool
}

func initDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	ddl := `
		CREATE SEQUENCE IF NOT EXISTS transfer_id_seq START 1;
		CREATE TABLE IF NOT EXISTS transfers (
			id              TEXT PRIMARY KEY,
			from_user       TEXT NOT NULL,
			source_account  TEXT NOT NULL,
			amount          TEXT NOT NULL,
			destination     TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'pending',
			quorum_request_id TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create schema/tables: %w", err)
	}

	return pool, nil
}

func newTransferStore(pool *pgxpool.Pool) *transferStore {
	return &transferStore{pool: pool}
}

const transferColumns = `id, from_user, source_account, amount, destination, status, COALESCE(quorum_request_id, ''), created_at`

func scanTransfer(row interface{ Scan(dest ...any) error }) (*Transfer, error) {
	t := &Transfer{}
	err := row.Scan(&t.ID, &t.FromUser, &t.SourceAccount, &t.Amount, &t.Destination, &t.Status, &t.QuorumRequestID, &t.CreatedAt)
	return t, err
}

func (s *transferStore) Create(ctx context.Context, t *Transfer) error {
	var seq int
	if err := s.pool.QueryRow(ctx, "SELECT nextval('transfer_id_seq')").Scan(&seq); err != nil {
		return fmt.Errorf("get next id: %w", err)
	}
	t.ID = fmt.Sprintf("TXN-%04d", seq)
	t.Status = StatusPending
	t.CreatedAt = time.Now().UTC()

	_, err := s.pool.Exec(ctx,
		`INSERT INTO transfers (id, from_user, source_account, amount, destination, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.ID, t.FromUser, t.SourceAccount, t.Amount, t.Destination, string(t.Status), t.CreatedAt,
	)
	return err
}

func (s *transferStore) Get(ctx context.Context, id string) (*Transfer, bool) {
	row := s.pool.QueryRow(ctx, `SELECT `+transferColumns+` FROM transfers WHERE id = $1`, id)
	t, err := scanTransfer(row)
	if err != nil {
		return nil, false
	}
	return t, true
}

func (s *transferStore) FindByQuorumID(ctx context.Context, quorumID string) (*Transfer, bool) {
	row := s.pool.QueryRow(ctx, `SELECT `+transferColumns+` FROM transfers WHERE quorum_request_id = $1`, quorumID)
	t, err := scanTransfer(row)
	if err != nil {
		return nil, false
	}
	return t, true
}

func (s *transferStore) SetStatus(ctx context.Context, id string, status TransferStatus) error {
	_, err := s.pool.Exec(ctx, `UPDATE transfers SET status = $1 WHERE id = $2`, string(status), id)
	return err
}

func (s *transferStore) SetQuorumRequestID(ctx context.Context, id, quorumRequestID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE transfers SET quorum_request_id = $1 WHERE id = $2`, quorumRequestID, id)
	return err
}

func (s *transferStore) List(ctx context.Context) []*Transfer {
	rows, err := s.pool.Query(ctx, `SELECT `+transferColumns+` FROM transfers ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("ERROR: failed to list transfers: %v", err)
		return nil
	}
	defer rows.Close()

	var transfers []*Transfer
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			log.Printf("ERROR: failed to scan transfer: %v", err)
			continue
		}
		transfers = append(transfers, t)
	}
	return transfers
}

// ---------------------------------------------------------------------------
// Application
// ---------------------------------------------------------------------------

type app struct {
	cfg    config
	store  *transferStore
	pages  map[string]*template.Template
	quorum *client.Client
}

func parseTemplates(basePath string) map[string]*template.Template {
	pages := map[string]*template.Template{}
	pageFiles := []string{"dashboard.html", "new_transfer.html", "detail.html"}
	funcs := template.FuncMap{"base": func() string { return basePath }}
	for _, page := range pageFiles {
		t := template.Must(template.New(page).Funcs(funcs).ParseFiles(
			"templates/layout.html",
			"templates/"+page,
		))
		pages[page] = t
	}
	return pages
}

func main() {
	cfg := loadConfig()

	ctx := context.Background()
	pool, err := initDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer pool.Close()

	quorum, err := client.New(cfg.QuorumAPIURL,
		client.WithTenant("banking"),
		client.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	)
	if err != nil {
		log.Fatalf("Invalid QUORUM_API_URL: %v", err)
	}

	a := &app{
		cfg:    cfg,
		store:  newTransferStore(pool),
		pages:  parseTemplates(cfg.BasePath),
		quorum: quorum,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("GET /transfer/new", a.handleNewTransferForm)
	mux.HandleFunc("POST /transfer/new", a.handleCreateTransfer)
	mux.HandleFunc("GET /transfer/{id}", a.handleTransferDetail)
	mux.HandleFunc("POST /webhooks/quorum", a.handleWebhook)
	mux.HandleFunc("POST /api/transfers", a.handleCreateTransferAPI)

	log.Printf("Banking app listening on :%s", cfg.Port)
	log.Printf("Quorum API: %s", cfg.QuorumAPIURL)
	log.Printf("Webhook callback: %s/webhooks/quorum", cfg.SelfURL)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

func (a *app) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"Transfers": a.store.List(r.Context()),
		"QuorumURL": a.cfg.QuorumPublicURL,
	}
	a.render(w, "dashboard.html", data)
}

func (a *app) handleNewTransferForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, "new_transfer.html", map[string]any{
		"QuorumURL": a.cfg.QuorumPublicURL,
	})
}

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

	transfer := &Transfer{
		FromUser:      fromUser,
		SourceAccount: sourceAccount,
		Amount:        amount,
		Destination:   destination,
	}
	if err := a.store.Create(r.Context(), transfer); err != nil {
		log.Printf("ERROR: failed to create transfer: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.submitToQuorum(r.Context(), transfer)
	http.Redirect(w, r, a.cfg.BasePath+"/transfer/"+transfer.ID, http.StatusSeeOther)
}

// handleCreateTransferAPI is a JSON endpoint used by the seed script.
func (a *app) handleCreateTransferAPI(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FromUser      string `json:"from_user"`
		SourceAccount string `json:"source_account"`
		Amount        string `json:"amount"`
		Destination   string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if input.FromUser == "" || input.SourceAccount == "" || input.Amount == "" || input.Destination == "" {
		http.Error(w, "all fields are required", http.StatusBadRequest)
		return
	}

	transfer := &Transfer{
		FromUser:      input.FromUser,
		SourceAccount: input.SourceAccount,
		Amount:        input.Amount,
		Destination:   input.Destination,
	}
	if err := a.store.Create(r.Context(), transfer); err != nil {
		log.Printf("ERROR: failed to create transfer: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.submitToQuorum(r.Context(), transfer)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(transfer)
}

func (a *app) submitToQuorum(ctx context.Context, transfer *Transfer) {
	// The transfer's creator is the maker. The idempotency key means a retry
	// of this call can never open a second approval for the same transfer.
	req, err := a.quorum.As(client.Identity{UserID: transfer.FromUser}).CreateRequest(ctx, client.CreateRequest{
		Type: "wire_transfer",
		Payload: map[string]string{
			"transfer_id":       transfer.ID,
			"from_user":         transfer.FromUser,
			"source_account_id": transfer.SourceAccount,
			"amount":            transfer.Amount,
			"destination":       transfer.Destination,
		},
		IdempotencyKey: "transfer-" + transfer.ID,
	})
	if err != nil {
		log.Printf("WARN: Quorum did not accept transfer %s: %v", transfer.ID, err)
		return
	}

	transfer.QuorumRequestID = req.ID.String()
	if err := a.store.SetQuorumRequestID(ctx, transfer.ID, transfer.QuorumRequestID); err != nil {
		log.Printf("ERROR: failed to save Quorum request ID: %v", err)
	}
	log.Printf("Transfer %s submitted to Quorum as request %s", transfer.ID, req.ID)
}

func (a *app) handleTransferDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	transfer, ok := a.store.Get(r.Context(), id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"Transfer":  transfer,
		"QuorumURL": a.cfg.QuorumPublicURL,
	}
	a.render(w, "detail.html", data)
}

// ---------------------------------------------------------------------------
// Webhook handler
// ---------------------------------------------------------------------------

func (a *app) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := client.ParseWebhook(r, a.cfg.WebhookSecret)
	if err != nil {
		if errors.Is(err, client.ErrInvalidSignature) {
			log.Println("WARN: webhook signature verification failed")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		log.Printf("WARN: failed to parse webhook payload: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	requestID := payload.Request.ID.String()
	transfer, ok := a.store.FindByQuorumID(ctx, requestID)
	if !ok {
		log.Printf("WARN: received webhook for unknown Quorum request %s", requestID)
		w.WriteHeader(http.StatusOK)
		return
	}

	switch payload.Event {
	case client.EventApproved:
		a.store.SetStatus(ctx, transfer.ID, StatusApproved)
		a.store.SetStatus(ctx, transfer.ID, StatusExecuted)
		log.Printf("Transfer %s approved and executed (Quorum request %s)", transfer.ID, requestID)
	case client.EventRejected:
		a.store.SetStatus(ctx, transfer.ID, StatusRejected)
		log.Printf("Transfer %s rejected (Quorum request %s)", transfer.ID, requestID)
	default:
		log.Printf("INFO: ignoring webhook event %q for request %s", payload.Event, requestID)
	}

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Template rendering
// ---------------------------------------------------------------------------

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
