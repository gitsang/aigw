package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	Health   HealthConfig   `yaml:"health"`
	Routing  RoutingConfig  `yaml:"routing"`
	Web      WebConfig      `yaml:"web"`
}

type ServerConfig struct {
	Address      string        `yaml:"address"`
	AdminToken   string        `yaml:"admin_token"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	CORSOrigins  []string      `yaml:"cors_origins"`
}

type DatabaseConfig struct {
	URL      string `yaml:"url"`
	MaxConns int32  `yaml:"max_conns"`
	MinConns int32  `yaml:"min_conns"`
	Migrate  bool   `yaml:"migrate"`
}

type SecurityConfig struct {
	EncryptionKey string `yaml:"encryption_key"`
}

type HealthConfig struct {
	Interval    time.Duration `yaml:"interval"`
	Timeout     time.Duration `yaml:"timeout"`
	Concurrency int           `yaml:"concurrency"`
}

type RoutingConfig struct {
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	MaxBodyBytes      int64         `yaml:"max_body_bytes"`
	DefaultRetryCount int           `yaml:"default_retry_count"`
	DefaultRetryDelay time.Duration `yaml:"default_retry_delay"`
	DefaultMaxBackoff time.Duration `yaml:"default_max_backoff"`
}

type WebConfig struct {
	Dir string `yaml:"dir"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Address:      ":8080",
			AdminToken:   "change-me",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  120 * time.Second,
			CORSOrigins:  []string{"http://localhost:5173", "http://localhost:8080"},
		},
		Database: DatabaseConfig{
			URL:      "postgres://aigw:aigw@localhost:5432/aigw?sslmode=disable",
			MaxConns: 20,
			MinConns: 2,
			Migrate:  true,
		},
		Security: SecurityConfig{EncryptionKey: "aigw-development-key-change-me"},
		Health:   HealthConfig{Interval: time.Minute, Timeout: 10 * time.Second, Concurrency: 8},
		Routing: RoutingConfig{
			RequestTimeout:    120 * time.Second,
			MaxBodyBytes:      32 << 20,
			DefaultRetryCount: 3,
			DefaultRetryDelay: 250 * time.Millisecond,
			DefaultMaxBackoff: 5 * time.Second,
		},
		Web: WebConfig{Dir: "./web/build"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return cfg, fmt.Errorf("read config: %w", err)
			}
		} else if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnv(&cfg)
	if cfg.Server.Address == "" {
		return cfg, errors.New("server.address must not be empty")
	}
	if cfg.Database.URL == "" {
		return cfg, errors.New("database.url must not be empty")
	}
	if cfg.Health.Interval <= 0 {
		return cfg, errors.New("health.interval must be positive")
	}
	if cfg.Health.Timeout <= 0 {
		cfg.Health.Timeout = 10 * time.Second
	}
	if cfg.Health.Concurrency <= 0 {
		cfg.Health.Concurrency = 8
	}
	if cfg.Routing.DefaultRetryCount < 1 {
		cfg.Routing.DefaultRetryCount = 1
	}
	if cfg.Routing.DefaultRetryDelay <= 0 {
		cfg.Routing.DefaultRetryDelay = 250 * time.Millisecond
	}
	if cfg.Routing.DefaultMaxBackoff <= 0 {
		cfg.Routing.DefaultMaxBackoff = 5 * time.Second
	}
	if cfg.Routing.MaxBodyBytes <= 0 {
		cfg.Routing.MaxBodyBytes = 32 << 20
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	setString := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok {
			*dst = v
		}
	}
	setDuration := func(key string, dst *time.Duration) {
		if v, ok := os.LookupEnv(key); ok {
			if d, err := time.ParseDuration(v); err == nil {
				*dst = d
			}
		}
	}
	setInt := func(key string, dst *int) {
		if v, ok := os.LookupEnv(key); ok {
			if n, err := strconv.Atoi(v); err == nil {
				*dst = n
			}
		}
	}
	setString("AIGW_SERVER_ADDRESS", &cfg.Server.Address)
	setString("AIGW_ADMIN_TOKEN", &cfg.Server.AdminToken)
	setString("AIGW_DATABASE_URL", &cfg.Database.URL)
	setString("AIGW_ENCRYPTION_KEY", &cfg.Security.EncryptionKey)
	setString("AIGW_WEB_DIR", &cfg.Web.Dir)
	setDuration("AIGW_HEALTH_INTERVAL", &cfg.Health.Interval)
	setDuration("AIGW_HEALTH_TIMEOUT", &cfg.Health.Timeout)
	setInt("AIGW_HEALTH_CONCURRENCY", &cfg.Health.Concurrency)
	setDuration("AIGW_REQUEST_TIMEOUT", &cfg.Routing.RequestTimeout)
	if v, ok := os.LookupEnv("AIGW_CORS_ORIGINS"); ok {
		var origins []string
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				origins = append(origins, p)
			}
		}
		cfg.Server.CORSOrigins = origins
	}
}
