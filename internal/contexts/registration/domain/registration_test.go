package domain_test

import (
	"errors"
	"testing"
	"time"

	"salt/hackethon/internal/contexts/registration/domain"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestNewRegistration(t *testing.T) {
	reg := domain.NewRegistration("Karyawan A", "a@corp.example", "code123", base, 10*time.Second)

	if reg.Status != domain.StatusPending {
		t.Errorf("status = %v, want pending", reg.Status)
	}
	if !reg.ExpiresAt.Equal(base.Add(10 * time.Second)) {
		t.Errorf("expiresAt = %v, want %v", reg.ExpiresAt, base.Add(10*time.Second))
	}
}

func TestRegistration_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status domain.Status
		now    time.Time
		want   bool
	}{
		{"success is always active", domain.StatusSuccess, base.Add(time.Hour), true},
		{"pending before expiry is active", domain.StatusPending, base.Add(5 * time.Second), true},
		{"pending at expiry is inactive", domain.StatusPending, base.Add(10 * time.Second), false},
		{"pending after expiry is inactive", domain.StatusPending, base.Add(11 * time.Second), false},
		{"expired is inactive", domain.StatusExpired, base, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := domain.NewRegistration("N", "e@corp.example", "code", base, 10*time.Second)
			reg.Status = tt.status
			if got := reg.IsActive(tt.now); got != tt.want {
				t.Errorf("IsActive(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestRegistration_Verify_Success(t *testing.T) {
	reg := domain.NewRegistration("N", "e@corp.example", "code", base, 10*time.Second)

	verifiedAt := base.Add(5 * time.Second)
	if err := reg.Verify(verifiedAt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Status != domain.StatusSuccess {
		t.Errorf("status = %v, want success", reg.Status)
	}
	if reg.VerifiedAt == nil || !reg.VerifiedAt.Equal(verifiedAt) {
		t.Errorf("verifiedAt = %v, want %v", reg.VerifiedAt, verifiedAt)
	}
}

func TestRegistration_Verify_Expired(t *testing.T) {
	reg := domain.NewRegistration("N", "e@corp.example", "code", base, 10*time.Second)

	err := reg.Verify(base.Add(10 * time.Second))
	if !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
	if reg.Status != domain.StatusExpired {
		t.Errorf("status = %v, want expired (must be persisted by caller)", reg.Status)
	}
}

func TestRegistration_Verify_AlreadySuccess_Idempotent(t *testing.T) {
	reg := domain.NewRegistration("N", "e@corp.example", "code", base, 10*time.Second)
	_ = reg.Verify(base.Add(time.Second))

	if err := reg.Verify(base.Add(2 * time.Second)); err != nil {
		t.Fatalf("unexpected error on re-verify: %v", err)
	}
	if reg.Status != domain.StatusSuccess {
		t.Errorf("status = %v, want success", reg.Status)
	}
}

func TestRegistration_Verify_AlreadyExpired(t *testing.T) {
	reg := domain.NewRegistration("N", "e@corp.example", "code", base, 10*time.Second)
	reg.Status = domain.StatusExpired

	err := reg.Verify(base.Add(time.Second))
	if !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := domain.NormalizeEmail("  Budi@Corp.Example  "); got != "budi@corp.example" {
		t.Errorf("NormalizeEmail = %q, want budi@corp.example", got)
	}
}
