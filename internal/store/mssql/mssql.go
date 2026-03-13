package mssql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lawale/quorum/internal/store"
	_ "github.com/microsoft/go-mssqldb"
)

type DB struct {
	Pool *sql.DB
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

	return &DB{Pool: db}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
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

func NewStores(ctx context.Context, dsn string, maxOpen, maxIdle int) (*store.Stores, error) {
	db, err := New(ctx, dsn, maxOpen, maxIdle)
	if err != nil {
		return nil, err
	}
	return &store.Stores{
		Requests:  NewRequestStore(db),
		Approvals: NewApprovalStore(db),
		Policies:  NewPolicyStore(db),
		Webhooks:  NewWebhookStore(db),
		Audits:    NewAuditStore(db),
		Operators: NewOperatorStore(db),
		Close:     db.Close,
	}, nil
}
