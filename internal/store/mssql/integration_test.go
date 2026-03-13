//go:build integration

package mssql_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lawale/quorum/internal/store/mssql"
	"github.com/lawale/quorum/internal/store/storetest"
	tcmssql "github.com/testcontainers/testcontainers-go/modules/mssql"
)

func migrationsDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations", "mssql")
}

func setupMSSQL(t *testing.T) *mssql.DB {
	t.Helper()
	ctx := context.Background()

	password := "SuperStrong@Passw0rd"

	ctr, err := tcmssql.Run(ctx, "mcr.microsoft.com/mssql/server:2022-CU14-ubuntu-22.04",
		tcmssql.WithAcceptEULA(),
		tcmssql.WithPassword(password),
	)
	if err != nil {
		t.Fatalf("start mssql container: %v", err)
	}
	t.Cleanup(func() { ctr.Terminate(context.Background()) })

	connStr, err := ctr.ConnectionString(ctx, "encrypt=false", "TrustServerCertificate=true")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	// Run migration scripts against the container
	migrationFiles := []string{
		"001_create_policies.up.sql",
		"002_create_requests.up.sql",
		"003_create_approvals.up.sql",
		"004_create_webhooks.up.sql",
		"005_create_audit_logs.up.sql",
		"006_add_permission_check_url.up.sql",
	}

	migDB, err := sql.Open("sqlserver", connStr)
	if err != nil {
		t.Fatalf("open migration connection: %v", err)
	}
	defer migDB.Close()

	for _, f := range migrationFiles {
		data, err := os.ReadFile(filepath.Join(migrationsDir(), f))
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := migDB.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("execute migration %s: %v", f, err)
		}
	}

	db, err := mssql.New(ctx, connStr, 5, 2)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)

	return db
}

func TestMSSQLRequestStore(t *testing.T) {
	db := setupMSSQL(t)
	storetest.TestRequestStore(t, mssql.NewRequestStore(db))
}

func TestMSSQLApprovalStore(t *testing.T) {
	db := setupMSSQL(t)
	storetest.TestApprovalStore(t, mssql.NewApprovalStore(db), mssql.NewRequestStore(db))
}

func TestMSSQLPolicyStore(t *testing.T) {
	db := setupMSSQL(t)
	storetest.TestPolicyStore(t, mssql.NewPolicyStore(db))
}

func TestMSSQLWebhookStore(t *testing.T) {
	db := setupMSSQL(t)
	storetest.TestWebhookStore(t, mssql.NewWebhookStore(db))
}

func TestMSSQLAuditStore(t *testing.T) {
	db := setupMSSQL(t)
	storetest.TestAuditStore(t, mssql.NewAuditStore(db), mssql.NewRequestStore(db))
}
