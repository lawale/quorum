package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Webhook  WebhookConfig  `yaml:"webhook"`
	Expiry   ExpiryConfig   `yaml:"expiry"`
	Metrics  MetricsConfig  `yaml:"metrics"`
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
	UserIDHeader string `yaml:"user_id_header"`
	RolesHeader  string `yaml:"roles_header"`
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
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`
	Timeout       time.Duration `yaml:"timeout"`
}

type ExpiryConfig struct {
	CheckInterval time.Duration `yaml:"check_interval"`
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

	return cfg, nil
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
	if cfg.Webhook.MaxRetries == 0 {
		cfg.Webhook.MaxRetries = 3
	}
	if cfg.Webhook.RetryInterval == 0 {
		cfg.Webhook.RetryInterval = 5 * time.Second
	}
	if cfg.Webhook.Timeout == 0 {
		cfg.Webhook.Timeout = 10 * time.Second
	}
	if cfg.Expiry.CheckInterval == 0 {
		cfg.Expiry.CheckInterval = time.Minute
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
}
