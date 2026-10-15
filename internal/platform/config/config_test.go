package config_test

import (
	"testing"
	"time"

	"salt/hackethon/internal/platform/config"
)

func TestLoad_Defaults(t *testing.T) {
	for _, key := range []string{
		"PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"WORKSHOP_QUOTA_TOTAL", "REGISTRATION_VERIFICATION_TTL_SECONDS",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM",
	} {
		t.Setenv(key, "")
	}

	cfg := config.Load()

	want := config.Config{
		Port: "8080", DBHost: "localhost", DBPort: "5432", DBUser: "postgres",
		DBPassword: "root", DBName: "hackethon", DBSSLMode: "disable",
		QuotaTotal:      30,
		VerificationTTL: 60 * time.Second,
		SMTPHost:        "smtp.gmail.com", SMTPPort: "587",
		SMTPUsername: "", SMTPPassword: "", SMTPFrom: "",
	}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_PORT", "5555")
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "workshop")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("WORKSHOP_QUOTA_TOTAL", "50")
	t.Setenv("REGISTRATION_VERIFICATION_TTL_SECONDS", "120")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "25")
	t.Setenv("SMTP_USERNAME", "bot@example.com")
	t.Setenv("SMTP_PASSWORD", "apppass")
	t.Setenv("SMTP_FROM", "")

	cfg := config.Load()

	if cfg.Port != "9090" || cfg.DBHost != "db.internal" || cfg.DBPort != "5555" ||
		cfg.DBUser != "app" || cfg.DBPassword != "secret" || cfg.DBName != "workshop" ||
		cfg.DBSSLMode != "require" {
		t.Errorf("DB config not overridden correctly: %+v", cfg)
	}
	if cfg.QuotaTotal != 50 {
		t.Errorf("QuotaTotal = %d, want 50", cfg.QuotaTotal)
	}
	if cfg.VerificationTTL != 120*time.Second {
		t.Errorf("VerificationTTL = %v, want 120s", cfg.VerificationTTL)
	}
	if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != "25" || cfg.SMTPUsername != "bot@example.com" || cfg.SMTPPassword != "apppass" {
		t.Errorf("SMTP config not overridden correctly: %+v", cfg)
	}
}

// TestLoad_SMTPFromDefaultsToUsername covers SMTP_FROM's fallback chain:
// when unset, it should default to SMTP_USERNAME rather than an empty string.
func TestLoad_SMTPFromDefaultsToUsername(t *testing.T) {
	t.Setenv("SMTP_USERNAME", "bot@example.com")
	t.Setenv("SMTP_FROM", "")

	cfg := config.Load()

	if cfg.SMTPFrom != "bot@example.com" {
		t.Errorf("SMTPFrom = %q, want bot@example.com (fallback to SMTP_USERNAME)", cfg.SMTPFrom)
	}
}

func TestLoad_SMTPFromExplicitOverride(t *testing.T) {
	t.Setenv("SMTP_USERNAME", "bot@example.com")
	t.Setenv("SMTP_FROM", "noreply@example.com")

	cfg := config.Load()

	if cfg.SMTPFrom != "noreply@example.com" {
		t.Errorf("SMTPFrom = %q, want noreply@example.com", cfg.SMTPFrom)
	}
}

// TestLoad_InvalidIntFallsBackToDefault covers getEnvInt's strconv.Atoi
// error path: a non-numeric value must not panic or propagate an error,
// it silently falls back to the default.
func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("WORKSHOP_QUOTA_TOTAL", "not-a-number")

	cfg := config.Load()

	if cfg.QuotaTotal != 30 {
		t.Errorf("QuotaTotal = %d, want 30 (fallback on invalid int)", cfg.QuotaTotal)
	}
}
