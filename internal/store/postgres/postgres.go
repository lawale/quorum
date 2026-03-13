package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lawale/quorum/internal/store"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string, maxOpen, maxIdle int) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing dsn: %w", err)
	}

	config.MaxConns = int32(maxOpen)
	config.MinConns = int32(maxIdle)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
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
