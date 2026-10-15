package database_test

import (
	"strings"
	"testing"

	"salt/hackethon/internal/platform/config"
	"salt/hackethon/internal/platform/database"
)

// TestNewPostgres_ConnectionError covers the error path deterministically:
// gorm.Open with the postgres driver connects eagerly, so an unreachable
// host fails fast with a DNS error rather than lazily succeeding. The
// success path requires a real Postgres instance and is covered by manual
// live-DB verification (see README) rather than a unit test.
func TestNewPostgres_ConnectionError(t *testing.T) {
	cfg := config.Config{
		DBHost: "nonexistent-host-for-unit-test.invalid",
		DBPort: "5432", DBUser: "x", DBPassword: "x", DBName: "x", DBSSLMode: "disable",
	}

	_, err := database.NewPostgres(cfg)
	if err == nil {
		t.Fatal("expected error connecting to an unreachable host, got nil")
	}
	if !strings.Contains(err.Error(), "connect postgres:") {
		t.Errorf("err = %v, want it wrapped with %q", err, "connect postgres:")
	}
}
