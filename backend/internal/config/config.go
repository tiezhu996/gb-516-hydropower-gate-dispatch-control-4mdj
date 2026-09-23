package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the complete runtime contract. Every value can be overridden by
// environment variables so the same binary works locally and in Compose.
type Config struct {
	AppName                   string
	Environment               string
	Port                      string
	DatabaseDriver            string
	DatabaseDSN               string
	RedisAddr                 string
	RedisPassword             string
	JWTSecret                 string
	BootstrapAdminPassword    string
	BootstrapOperatorPassword string
	BootstrapReviewerPassword string
	TokenTTL                  time.Duration
	RequestLimit              int
	LoginRequestLimit         int
	TrustedProxies            []string
	StartupTimeout            time.Duration
	ShutdownTimeout           time.Duration
	ReadHeaderTimeout         time.Duration
	ReadTimeout               time.Duration
	WriteTimeout              time.Duration
	IdleTimeout               time.Duration
}

// PublicConfig is safe to expose from the authenticated runtime endpoint.
// Connection strings, passwords and signing keys are intentionally omitted.
type PublicConfig struct {
	AppName           string `json:"appName"`
	Environment       string `json:"environment"`
	DatabaseDriver    string `json:"databaseDriver"`
	RedisEnabled      bool   `json:"redisEnabled"`
	TokenTTLSeconds   int64  `json:"tokenTtlSeconds"`
	RequestLimit      int    `json:"requestLimit"`
	LoginRequestLimit int    `json:"loginRequestLimit"`
	ShutdownSeconds   int64  `json:"shutdownSeconds"`
}

func Load() (Config, error) {
	cfg := Config{
		AppName:                   env("APP_NAME", "hydropower-gate-dispatch-control"),
		Environment:               strings.ToLower(env("APP_ENV", "development")),
		Port:                      env("PORT", "8080"),
		DatabaseDriver:            strings.ToLower(env("DATABASE_DRIVER", "postgres")),
		DatabaseDSN:               env("DATABASE_DSN", "app.db"),
		RedisAddr:                 env("REDIS_ADDR", ""),
		RedisPassword:             env("REDIS_PASSWORD", ""),
		JWTSecret:                 env("JWT_SECRET", "development-only-change-me"),
		BootstrapAdminPassword:    env("BOOTSTRAP_ADMIN_PASSWORD", "Admin123!"),
		BootstrapOperatorPassword: env("BOOTSTRAP_OPERATOR_PASSWORD", "Admin123!"),
		BootstrapReviewerPassword: env("BOOTSTRAP_REVIEWER_PASSWORD", "Admin123!"),
		TokenTTL:                  durationEnv("TOKEN_TTL", 8*time.Hour),
		RequestLimit:              intEnv("REQUEST_LIMIT", 180),
		LoginRequestLimit:         intEnv("LOGIN_REQUEST_LIMIT", 8),
		TrustedProxies:            splitCSV(env("TRUSTED_PROXIES", "")),
		StartupTimeout:            durationEnv("STARTUP_TIMEOUT", 35*time.Second),
		ShutdownTimeout:           durationEnv("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout:         durationEnv("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:               durationEnv("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:              durationEnv("WRITE_TIMEOUT", 20*time.Second),
		IdleTimeout:               durationEnv("IDLE_TIMEOUT", 60*time.Second),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) ListenAddress() string { return ":" + c.Port }
func (c Config) IsProduction() bool    { return c.Environment == "production" }

func (c Config) RedisEnabled() bool { return strings.TrimSpace(c.RedisAddr) != "" }

func (c Config) Public() PublicConfig {
	return PublicConfig{
		AppName: c.AppName, Environment: c.Environment, DatabaseDriver: c.DatabaseDriver,
		RedisEnabled: c.RedisEnabled(), TokenTTLSeconds: int64(c.TokenTTL.Seconds()),
		RequestLimit: c.RequestLimit, LoginRequestLimit: c.LoginRequestLimit,
		ShutdownSeconds: int64(c.ShutdownTimeout.Seconds()),
	}
}

func (c Config) Validate() error {
	switch c.DatabaseDriver {
	case "postgres", "mysql", "sqlite":
	default:
		return fmt.Errorf("unsupported DATABASE_DRIVER %q", c.DatabaseDriver)
	}
	if strings.TrimSpace(c.AppName) == "" {
		return fmt.Errorf("APP_NAME must not be empty")
	}
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("PORT must not be empty")
	}
	if len(c.JWTSecret) < 16 {
		return fmt.Errorf("JWT_SECRET must contain at least 16 characters")
	}
	productionSecret := strings.ToLower(c.JWTSecret)
	if c.IsProduction() && (c.JWTSecret == "development-only-change-me" || len(c.JWTSecret) < 32 || strings.Contains(productionSecret, "replace-this") || strings.Contains(productionSecret, "change-me")) {
		return fmt.Errorf("production JWT_SECRET must be unique and contain at least 32 characters")
	}
	if c.IsProduction() && (c.BootstrapAdminPassword == "Admin123!" || len(c.BootstrapAdminPassword) < 12) {
		return fmt.Errorf("production BOOTSTRAP_ADMIN_PASSWORD must be unique and contain at least 12 characters")
	}
	if c.IsProduction() && (c.BootstrapOperatorPassword == "Admin123!" || len(c.BootstrapOperatorPassword) < 12) {
		return fmt.Errorf("production BOOTSTRAP_OPERATOR_PASSWORD must be unique and contain at least 12 characters")
	}
	if c.IsProduction() && (c.BootstrapReviewerPassword == "Admin123!" || len(c.BootstrapReviewerPassword) < 12) {
		return fmt.Errorf("production BOOTSTRAP_REVIEWER_PASSWORD must be unique and contain at least 12 characters")
	}
	if c.IsProduction() && (c.BootstrapAdminPassword == c.BootstrapOperatorPassword || c.BootstrapAdminPassword == c.BootstrapReviewerPassword || c.BootstrapOperatorPassword == c.BootstrapReviewerPassword) {
		return fmt.Errorf("production bootstrap account passwords must be distinct")
	}
	if c.RequestLimit < 10 || c.RequestLimit > 100000 {
		return fmt.Errorf("REQUEST_LIMIT must be between 10 and 100000")
	}
	if c.LoginRequestLimit < 1 || c.LoginRequestLimit > 1000 {
		return fmt.Errorf("LOGIN_REQUEST_LIMIT must be between 1 and 1000")
	}
	for name, value := range map[string]time.Duration{
		"STARTUP_TIMEOUT": c.StartupTimeout, "SHUTDOWN_TIMEOUT": c.ShutdownTimeout,
		"READ_HEADER_TIMEOUT": c.ReadHeaderTimeout, "READ_TIMEOUT": c.ReadTimeout,
		"WRITE_TIMEOUT": c.WriteTimeout, "IDLE_TIMEOUT": c.IdleTimeout,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	values := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
