package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lawale/quorum/internal/store"
)

// DBTX is the common query interface shared by *pgxpool.Pool and pgx.Tx.
// All postgres stores use this interface for their queries, allowing them
// to operate on either a connection pool or a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type DB struct {
	pool *pgxpool.Pool // private: for BeginTx, Close, Ping
	Pool DBTX          // public: used by stores — can be pool or tx
}

func New(ctx context.Context, dsn string, maxOpen, maxIdle int) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing dsn: %w", err)
	}

	config.MaxConns = int32(maxOpen)
	config.MinConns = int32(maxIdle)

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO quorum, public")
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{pool: pool, Pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

// withTx returns a new DB that uses the given transaction for all queries.
func (db *DB) withTx(tx pgx.Tx) *DB {
	return &DB{pool: db.pool, Pool: tx}
}

func NewStores(ctx context.Context, dsn string, maxOpen, maxIdle int) (*store.Stores, error) {
	db, err := New(ctx, dsn, maxOpen, maxIdle)
	if err != nil {
		return nil, err
	}

	s := &store.Stores{
		Requests:  NewRequestStore(db),
		Approvals: NewApprovalStore(db),
		Policies:  NewPolicyStore(db),
		Webhooks:  NewWebhookStore(db),
		Audits:    NewAuditStore(db),
		Operators: NewOperatorStore(db),
		Tenants:   NewTenantStore(db),
		Outbox:    NewOutboxStore(db),
		Close:     db.Close,
	}

	s.RunInTx = func(ctx context.Context, fn func(tx *store.Stores) error) error {
		tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		txDB := db.withTx(tx)
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

		return tx.Commit(ctx)
	}

	return s, nil
}
