package migrate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/sqlserver"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/config"
)

func Run(cfg config.DatabaseConfig, sourcePath, command string, steps int) error {
	// For Postgres, ensure the quorum schema exists before initializing
	// the migration driver. The pgx/v5 driver queries CURRENT_SCHEMA()
	// on init, which returns NULL if the schema in search_path doesn't
	// exist yet. Pre-creating it avoids that error and ensures the
	// schema_migrations table lives in the quorum schema (not public).
	if cfg.Driver != "mssql" {
		if err := ensureSchema(cfg.DSN()); err != nil {
			return fmt.Errorf("ensuring schema: %w", err)
		}
	}

	dsn := migrationDSN(cfg)

	m, err := migrate.New(sourcePath, dsn)
	if err != nil {
		return fmt.Errorf("initializing migrate: %w", err)
	}
	defer m.Close()

	switch command {
	case "up":
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				slog.Info("migrations: no changes to apply")
				return nil
			}
			return fmt.Errorf("applying migrations: %w", err)
		}
		slog.Info("migrations applied successfully")

	case "down":
		if steps <= 0 {
			steps = 1
		}
		if err := m.Steps(-steps); err != nil {
			return fmt.Errorf("rolling back migrations: %w", err)
		}
		slog.Info("migrations rolled back", "steps", steps)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			return fmt.Errorf("getting migration version: %w", err)
		}
		slog.Info("migration version", "version", version, "dirty", dirty)

	default:
		return fmt.Errorf("unknown migration command: %q (expected up, down, or version)", command)
	}

	return nil
}

// ensureSchema creates the quorum schema if it doesn't exist. This must
// run before golang-migrate initializes so that search_path=quorum
// resolves and CURRENT_SCHEMA() returns "quorum".
func ensureSchema(dsn string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS quorum")
	return err
}

func migrationDSN(cfg config.DatabaseConfig) string {
	dsn := cfg.DSN()
	if cfg.Driver == "mssql" {
		return dsn
	}
	// The pgx/v5 driver registers as "pgx5", so replace the postgres:// scheme.
	dsn = strings.Replace(dsn, "postgres://", "pgx5://", 1)
	// Set search_path so the schema_migrations table is created in the
	// quorum schema, avoiding conflicts with other apps on the same database.
	if strings.Contains(dsn, "?") {
		if !strings.Contains(dsn, "search_path=") {
			dsn += "&search_path=quorum"
		}
	} else {
		dsn += "?search_path=quorum"
	}
	return dsn
}
