package config

import "testing"

func TestLoadRejectsUnknownDatabaseDriver(t *testing.T) {
	t.Setenv("DATABASE_DRIVER", "unknown")
	if _, err := Load(); err == nil {
		t.Fatal("expected unsupported database driver to fail")
	}
}

func TestLoadAcceptsSQLiteForLocalSmoke(t *testing.T) {
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", ":memory:")
	t.Setenv("JWT_SECRET", "this-is-long-enough-for-tests")
	if _, err := Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}
}

func TestLoadRejectsDevelopmentSecretInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "development-only-change-me")
	if _, err := Load(); err == nil {
		t.Fatal("production must reject the development signing secret")
	}
}

func TestLoadRequiresLongProductionSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "unique-but-too-short")
	if _, err := Load(); err == nil {
		t.Fatal("production must require at least 32 signing-secret characters")
	}
}

func TestLoadRejectsDefaultBootstrapPasswordInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "a-unique-production-signing-secret-with-40-characters")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "Admin123!")
	if _, err := Load(); err == nil {
		t.Fatal("production must reject the default bootstrap administrator password")
	}
}
