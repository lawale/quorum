//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lawale/quorum/internal/store/postgres"
	"github.com/lawale/quorum/internal/store/storetest"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func migrationsDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations", "postgres")
}

func setupPostgres(t *testing.T) *postgres.DB {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("quorum_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.WithInitScripts(
			filepath.Join(migrationsDir(), "001_create_tenants.up.sql"),
			filepath.Join(migrationsDir(), "002_create_policies.up.sql"),
			filepath.Join(migrationsDir(), "003_create_requests.up.sql"),
			filepath.Join(migrationsDir(), "004_create_approvals.up.sql"),
			filepath.Join(migrationsDir(), "005_create_webhooks.up.sql"),
			filepath.Join(migrationsDir(), "006_create_audit_logs.up.sql"),
			filepath.Join(migrationsDir(), "007_create_operators.up.sql"),
			filepath.Join(migrationsDir(), "008_create_webhook_outbox.up.sql"),
		),
		tcpostgres.BasicWaitStrategies(),
		tcpostgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { ctr.Terminate(context.Background()) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	db, err := postgres.New(ctx, dsn, 5, 2)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)

	return db
}

func TestPostgresRequestStore(t *testing.T) {
	db := setupPostgres(t)
	storetest.TestRequestStore(t, postgres.NewRequestStore(db))
}

func TestPostgresApprovalStore(t *testing.T) {
	db := setupPostgres(t)
	storetest.TestApprovalStore(t, postgres.NewApprovalStore(db), postgres.NewRequestStore(db))
}

func TestPostgresPolicyStore(t *testing.T) {
	db := setupPostgres(t)
	storetest.TestPolicyStore(t, postgres.NewPolicyStore(db))
}

func TestPostgresWebhookStore(t *testing.T) {
	db := setupPostgres(t)
	storetest.TestWebhookStore(t, postgres.NewWebhookStore(db))
}

func TestPostgresAuditStore(t *testing.T) {
	db := setupPostgres(t)
	storetest.TestAuditStore(t, postgres.NewAuditStore(db), postgres.NewRequestStore(db))
}

func TestPostgresOperatorStore(t *testing.T) {
	db := setupPostgres(t)
	storetest.TestOperatorStore(t, postgres.NewOperatorStore(db))
}
