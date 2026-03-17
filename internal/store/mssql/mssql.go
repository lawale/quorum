package mssql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lawale/quorum/internal/health"
	"github.com/lawale/quorum/internal/store"
	_ "github.com/microsoft/go-mssqldb"
)

// DBTX is the common query interface shared by *sql.DB and *sql.Tx.
// All mssql stores use this interface for their queries, allowing them
// to operate on either a connection pool or a transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type DB struct {
	pool *sql.DB // private: for BeginTx, Close, Ping
	Pool DBTX    // public: used by stores — can be pool or tx
}

func New(ctx context.Context, dsn string, maxOpen, maxIdle int) (*DB, error) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{pool: db, Pool: db}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Name() string                     { return "mssql" }
func (db *DB) Health(ctx context.Context) error { return db.pool.PingContext(ctx) }

// withTx returns a new DB that uses the given transaction for all queries.
func (db *DB) withTx(tx *sql.Tx) *DB {
	return &DB{pool: db.pool, Pool: tx}
}

// nullableString converts a byte slice to a *string, returning nil if empty.
// Used for nullable NVARCHAR(MAX) columns in MSSQL that store JSON data.
func nullableString(data []byte) *string {
	if len(data) == 0 {
		return nil
	}
	s := string(data)
	return &s
}

// NewStoresFromDB builds a Stores bundle from an existing DB connection.
// Used by integration tests that manage their own database lifecycle.
func NewStoresFromDB(db *DB) (*store.Stores, error) {
	s := &store.Stores{
		Requests:       NewRequestStore(db),
		Approvals:      NewApprovalStore(db),
		Policies:       NewPolicyStore(db),
		Webhooks:       NewWebhookStore(db),
		Audits:         NewAuditStore(db),
		Operators:      NewOperatorStore(db),
		Tenants:        NewTenantStore(db),
		Outbox:         NewOutboxStore(db),
		Close:          func() {}, // caller owns the DB lifecycle
		HealthCheckers: []health.HealthChecker{db},
	}

	s.RunInTx = func(ctx context.Context, fn func(tx *store.Stores) error) error {
		sqlTx, err := db.pool.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}
		defer sqlTx.Rollback() //nolint:errcheck

		txDB := db.withTx(sqlTx)
		txStores := &store.Stores{
			Requests:  NewRequestStore(txDB),
			Approvals: NewApprovalStore(txDB),
			Policies:  NewPolicyStore(txDB),
			Webhooks:  NewWebhookStore(txDB),
			Audits:    NewAuditStore(txDB),
			Operators: NewOperatorStore(txDB),
			Tenants:   NewTenantStore(txDB),
			Outbox:    NewOutboxStore(txDB),
		}

		if err := fn(txStores); err != nil {
			return err
		}

		return sqlTx.Commit()
	}

	return s, nil
}

func NewStores(ctx context.Context, dsn string, maxOpen, maxIdle int) (*store.Stores, error) {
	db, err := New(ctx, dsn, maxOpen, maxIdle)
	if err != nil {
		return nil, err
	}

	s := &store.Stores{
		Requests:       NewRequestStore(db),
		Approvals:      NewApprovalStore(db),
		Policies:       NewPolicyStore(db),
		Webhooks:       NewWebhookStore(db),
		Audits:         NewAuditStore(db),
		Operators:      NewOperatorStore(db),
		Tenants:        NewTenantStore(db),
		Outbox:         NewOutboxStore(db),
		Close:          db.Close,
		HealthCheckers: []health.HealthChecker{db},
	}

	s.RunInTx = func(ctx context.Context, fn func(tx *store.Stores) error) error {
		sqlTx, err := db.pool.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}
		defer sqlTx.Rollback() //nolint:errcheck

		txDB := db.withTx(sqlTx)
		txStores := &store.Stores{
			Requests:  NewRequestStore(txDB),
			Approvals: NewApprovalStore(txDB),
			Policies:  NewPolicyStore(txDB),
			Webhooks:  NewWebhookStore(txDB),
			Audits:    NewAuditStore(txDB),
			Operators: NewOperatorStore(txDB),
			Tenants:   NewTenantStore(txDB),
			Outbox:    NewOutboxStore(txDB),
		}

		if err := fn(txStores); err != nil {
			return err
		}

		return sqlTx.Commit()
	}

	return s, nil
}
