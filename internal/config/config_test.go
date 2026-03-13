package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig_Load_ValidFile(t *testing.T) {
	content := `
server:
  host: "127.0.0.1"
  port: 9090
database:
  host: "db.example.com"
  port: 5432
  user: "testuser"
  password: "testpass"
  name: "testdb"
auth:
  mode: "trust"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.example.com")
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Database.User = %q, want %q", cfg.Database.User, "testuser")
	}
}

func TestConfig_Load_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestConfig_Load_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	content := `{}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("default Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("default Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("default SSLMode = %q, want %q", cfg.Database.SSLMode, "disable")
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("default MaxOpenConns = %d, want %d", cfg.Database.MaxOpenConns, 25)
	}
	if cfg.Database.MaxIdleConns != 5 {
		t.Errorf("default MaxIdleConns = %d, want %d", cfg.Database.MaxIdleConns, 5)
	}
	if cfg.Auth.Mode != "trust" {
		t.Errorf("default Auth.Mode = %q, want %q", cfg.Auth.Mode, "trust")
	}
	if cfg.Auth.Trust.UserIDHeader != "X-User-ID" {
		t.Errorf("default UserIDHeader = %q, want %q", cfg.Auth.Trust.UserIDHeader, "X-User-ID")
	}
	if cfg.Auth.Trust.RolesHeader != "X-User-Roles" {
		t.Errorf("default RolesHeader = %q, want %q", cfg.Auth.Trust.RolesHeader, "X-User-Roles")
	}
	if cfg.Webhook.MaxRetries != 3 {
		t.Errorf("default MaxRetries = %d, want %d", cfg.Webhook.MaxRetries, 3)
	}
	if cfg.Webhook.RetryInterval != 5*time.Second {
		t.Errorf("default RetryInterval = %v, want %v", cfg.Webhook.RetryInterval, 5*time.Second)
	}
	if cfg.Webhook.Timeout != 10*time.Second {
		t.Errorf("default Timeout = %v, want %v", cfg.Webhook.Timeout, 10*time.Second)
	}
	if cfg.Expiry.CheckInterval != time.Minute {
		t.Errorf("default CheckInterval = %v, want %v", cfg.Expiry.CheckInterval, time.Minute)
	}
}

func TestServerConfig_Addr(t *testing.T) {
	cfg := ServerConfig{Host: "localhost", Port: 3000}
	got := cfg.Addr()
	if got != "localhost:3000" {
		t.Errorf("Addr() = %q, want %q", got, "localhost:3000")
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "quorum",
		Password: "secret",
		Name:     "quorum_db",
		SSLMode:  "disable",
	}
	got := cfg.DSN()
	expected := "postgres://quorum:secret@localhost:5432/quorum_db?sslmode=disable"
	if got != expected {
		t.Errorf("DSN() = %q, want %q", got, expected)
	}
}
