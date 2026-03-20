package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Webhook  WebhookConfig  `yaml:"webhook"`
	Expiry   ExpiryConfig   `yaml:"expiry"`
	Tenant   TenantConfig   `yaml:"tenant"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Console  ConsoleConfig  `yaml:"console"`
}

type ConsoleConfig struct {
	Enabled       bool              `yaml:"enabled"`
	JWTSecret     string            `yaml:"jwt_secret"`
	SecureCookies bool              `yaml:"secure_cookies"`
	Suggestions   SuggestionsConfig `yaml:"suggestions"`
}

type SuggestionsConfig struct {
	RolesURL       string `yaml:"roles_url"`
	PermissionsURL string `yaml:"permissions_url"`
	AuthHeader     string `yaml:"auth_header"`
	AuthValue      string `yaml:"auth_value"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Driver       string            `yaml:"driver"`
	Host         string            `yaml:"host"`
	Port         int               `yaml:"port"`
	User         string            `yaml:"user"`
	Password     string            `yaml:"password"`
	Name         string            `yaml:"name"`
	Params       map[string]string `yaml:"params"`
	MaxOpenConns int               `yaml:"max_open_conns"`
	MaxIdleConns int               `yaml:"max_idle_conns"`
}

func (d DatabaseConfig) DSN() string {
	params := url.Values{}
	for k, v := range d.Params {
		params.Set(k, v)
	}
	query := params.Encode()

	switch d.Driver {
	case "mssql":
		if query != "" {
			query = "&" + query
		}
		return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s%s",
			d.User, d.Password, d.Host, d.Port, d.Name, query)
	default:
		if query != "" {
			query = "?" + query
		}
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s%s",
			d.User, d.Password, d.Host, d.Port, d.Name, query)
	}
}

type AuthConfig struct {
	Mode   string       `yaml:"mode"`
	Trust  TrustConfig  `yaml:"trust"`
	Verify VerifyConfig `yaml:"verify"`
	Custom CustomConfig `yaml:"custom"`
}

type TrustConfig struct {
	UserIDHeader      string `yaml:"user_id_header"`
	RolesHeader       string `yaml:"roles_header"`
	PermissionsHeader string `yaml:"permissions_header"`
	TenantIDHeader    string `yaml:"tenant_id_header"`
}

type VerifyConfig struct {
	JWKSURL  string       `yaml:"jwks_url"`
	Issuer   string       `yaml:"issuer"`
	Audience string       `yaml:"audience"`
	Claims   ClaimsConfig `yaml:"claims"`
}

type ClaimsConfig struct {
	UserID string `yaml:"user_id"`
	Roles  string `yaml:"roles"`
}

type CustomConfig struct {
	Endpoint string        `yaml:"endpoint"`
	Timeout  time.Duration `yaml:"timeout"`
}

type WebhookConfig struct {
	MaxRetries      int           `yaml:"max_retries"`
	RetryInterval   time.Duration `yaml:"retry_interval"`
	RetryWindow     time.Duration `yaml:"retry_window"`
	MaxRetryDelay   time.Duration `yaml:"max_retry_delay"`
	Timeout         time.Duration `yaml:"timeout"`
	Heartbeat       time.Duration `yaml:"heartbeat"`
	OutboxRetention time.Duration `yaml:"outbox_retention"`
	BlockPrivateIPs bool          `yaml:"block_private_ips"`
}

type ExpiryConfig struct {
	CheckInterval time.Duration `yaml:"check_interval"`
}

type TenantConfig struct {
	CacheRefreshInterval time.Duration `yaml:"cache_refresh_interval"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	setDefaults(cfg)
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides walks the Config struct tree via reflection and applies
// environment variable overrides. Env var names are derived from the struct
// path: Config.Server.Host → QUORUM_SERVER_HOST. CamelCase field names are
// converted to SCREAMING_SNAKE_CASE (e.g., MaxOpenConns → MAX_OPEN_CONNS).
func applyEnvOverrides(cfg *Config) {
	walkStruct(reflect.ValueOf(cfg).Elem(), []string{"QUORUM"})
}

func walkStruct(v reflect.Value, prefix []string) {
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fv := v.Field(i)

		if !field.IsExported() {
			continue
		}

		segment := camelToScreamingSnake(field.Name)
		path := append(append([]string{}, prefix...), segment)

		switch fv.Kind() {
		case reflect.Struct:
			if fv.Type() == reflect.TypeFor[time.Duration]() {
				applyEnvValue(fv, strings.Join(path, "_"))
			} else {
				walkStruct(fv, path)
			}
		case reflect.Map:
			continue
		default:
			applyEnvValue(fv, strings.Join(path, "_"))
		}
	}
}

func applyEnvValue(fv reflect.Value, envKey string) {
	val := os.Getenv(envKey)
	if val == "" {
		return
	}

	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Int, reflect.Int64:
		if fv.Type() == reflect.TypeFor[time.Duration]() {
			d, err := time.ParseDuration(val)
			if err != nil {
				slog.Warn("ignoring invalid duration env var", "key", envKey, "value", val, "error", err)
				return
			}
			fv.Set(reflect.ValueOf(d))
		} else {
			n, err := strconv.Atoi(val)
			if err != nil {
				slog.Warn("ignoring invalid int env var", "key", envKey, "value", val, "error", err)
				return
			}
			fv.SetInt(int64(n))
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			slog.Warn("ignoring invalid bool env var", "key", envKey, "value", val, "error", err)
			return
		}
		fv.SetBool(b)
	}
}

// camelToScreamingSnake converts a CamelCase identifier to SCREAMING_SNAKE_CASE.
// Examples: "MaxOpenConns" → "MAX_OPEN_CONNS", "Host" → "HOST", "JWTSecret" → "JWT_SECRET".
func camelToScreamingSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			if unicode.IsLower(prev) {
				b.WriteRune('_')
			} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				b.WriteRune('_')
			}
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "postgres"
	}
	if cfg.Database.Params == nil {
		cfg.Database.Params = make(map[string]string)
	}
	if cfg.Database.Port == 0 {
		switch cfg.Database.Driver {
		case "mssql":
			cfg.Database.Port = 1433
		default:
			cfg.Database.Port = 5432
		}
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Auth.Mode == "" {
		cfg.Auth.Mode = "trust"
	}
	if cfg.Auth.Trust.UserIDHeader == "" {
		cfg.Auth.Trust.UserIDHeader = "X-User-ID"
	}
	if cfg.Auth.Trust.RolesHeader == "" {
		cfg.Auth.Trust.RolesHeader = "X-User-Roles"
	}
	if cfg.Auth.Trust.PermissionsHeader == "" {
		cfg.Auth.Trust.PermissionsHeader = "X-User-Permissions"
	}
	if cfg.Auth.Trust.TenantIDHeader == "" {
		cfg.Auth.Trust.TenantIDHeader = "X-Tenant-ID"
	}
	if cfg.Webhook.MaxRetries == 0 {
		cfg.Webhook.MaxRetries = 20
	}
	if cfg.Webhook.RetryInterval == 0 {
		cfg.Webhook.RetryInterval = 30 * time.Second
	}
	if cfg.Webhook.RetryWindow == 0 {
		cfg.Webhook.RetryWindow = 72 * time.Hour
	}
	if cfg.Webhook.MaxRetryDelay == 0 {
		cfg.Webhook.MaxRetryDelay = time.Hour
	}
	if cfg.Webhook.Timeout == 0 {
		cfg.Webhook.Timeout = 10 * time.Second
	}
	if cfg.Expiry.CheckInterval == 0 {
		cfg.Expiry.CheckInterval = time.Minute
	}
	if cfg.Tenant.CacheRefreshInterval == 0 {
		cfg.Tenant.CacheRefreshInterval = time.Minute
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
}
