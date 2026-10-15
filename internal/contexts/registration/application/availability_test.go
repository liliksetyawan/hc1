package application_test

import (
	"context"
	"testing"
	"time"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/domain"
)

func TestAvailabilityHandler(t *testing.T) {
	repo := newFakeRepository()
	registerHandler := application.NewRegisterHandler(repo, 30, longTTL)
	availabilityHandler := application.NewAvailabilityHandler(repo, 30)

	for i := 0; i < 12; i++ {
		if _, err := registerHandler.Handle(context.Background(), application.RegisterCommand{
			Name:  "Seed",
			Email: seedEmail(i),
		}); err != nil {
			t.Fatalf("seed registration %d: %v", i, err)
		}
	}

	result, err := availabilityHandler.Handle(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := application.Availability{Total: 30, Registered: 12, Remaining: 18}
	if *result != want {
		t.Errorf("availability = %+v, want %+v", *result, want)
	}
}

func TestAvailabilityHandler_RemainingNeverNegative(t *testing.T) {
	repo := newFakeRepository()
	registerHandler := application.NewRegisterHandler(repo, 1, longTTL)
	availabilityHandler := application.NewAvailabilityHandler(repo, 1)

	if _, err := registerHandler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan C",
		Email: "c@corp.example",
	}); err != nil {
		t.Fatalf("registration: %v", err)
	}

	result, err := availabilityHandler.Handle(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", result.Remaining)
	}
}

// TestAvailabilityHandler_SweepsExpiredRowStatus verifies that checking
// availability also opportunistically persists status="expired" on stale
// pending rows, same as registering does — so the status column stays
// accurate even if nobody registers again for a while.
func TestAvailabilityHandler_SweepsExpiredRowStatus(t *testing.T) {
	repo := newFakeRepository()
	registerHandler := application.NewRegisterHandler(repo, 30, 30*time.Millisecond)
	availabilityHandler := application.NewAvailabilityHandler(repo, 30)

	reg, err := registerHandler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan H",
		Email: "h@corp.example",
	})
	if err != nil {
		t.Fatalf("registration: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := availabilityHandler.Handle(context.Background()); err != nil {
		t.Fatalf("availability: %v", err)
	}

	stale, err := repo.FindByCode(context.Background(), reg.RegisCode)
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if stale.Status != domain.StatusExpired {
		t.Errorf("status = %q, want %q (availability check should have swept it)", stale.Status, domain.StatusExpired)
	}
}
