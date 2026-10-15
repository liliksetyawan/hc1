package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/domain"
)

const longTTL = time.Hour

func TestRegisterHandler_Success(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 30, longTTL)

	for i := 0; i < 28; i++ {
		if _, err := handler.Handle(context.Background(), application.RegisterCommand{
			Name:  "Seed",
			Email: seedEmail(i),
		}); err != nil {
			t.Fatalf("seed registration %d: %v", i, err)
		}
	}

	result, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan A",
		Email: "a@corp.example",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RegistrationID != "REG-0029" {
		t.Errorf("registrationId = %q, want REG-0029", result.RegistrationID)
	}
	if result.Status != "pending" {
		t.Errorf("status = %q, want pending", result.Status)
	}
	if result.RegisCode == "" {
		t.Error("regisCode is empty, want a generated code")
	}
	if result.Availability.Registered != 29 || result.Availability.Remaining != 1 || result.Availability.Total != 30 {
		t.Errorf("availability = %+v, want {30 29 1}", result.Availability)
	}
}

func TestRegisterHandler_NormalizesEmailForStorageAndDuplicateCheck(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 30, longTTL)

	if _, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Budi",
		Email: "budi@corp.example",
	}); err != nil {
		t.Fatalf("first registration: %v", err)
	}

	_, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Budi",
		Email: " Budi@Corp.Example ",
	})
	if !errors.Is(err, application.ErrEmailAlreadyRegistered) {
		t.Fatalf("err = %v, want ErrEmailAlreadyRegistered", err)
	}
}

func TestRegisterHandler_QuotaFull(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 1, longTTL)

	if _, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan C",
		Email: "c@corp.example",
	}); err != nil {
		t.Fatalf("first registration: %v", err)
	}

	_, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan D",
		Email: "d@corp.example",
	})
	if !errors.Is(err, application.ErrQuotaFull) {
		t.Fatalf("err = %v, want ErrQuotaFull", err)
	}
}

func TestRegisterHandler_ValidationError(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 30, longTTL)

	_, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "",
		Email: "abc",
	})

	var valErr *application.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if len(valErr.Fields) != 2 {
		t.Fatalf("fields = %+v, want 2 entries", valErr.Fields)
	}
	if valErr.Fields[0].Field != "name" || valErr.Fields[1].Field != "email" {
		t.Errorf("fields = %+v, want name then email", valErr.Fields)
	}
}

// TestRegisterHandler_ExpiredPendingFreesSlot verifies the brief's requirement:
// an unverified registration must give its quota slot back once its TTL passes.
func TestRegisterHandler_ExpiredPendingFreesSlot(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 1, 30*time.Millisecond)

	if _, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan E",
		Email: "e@corp.example",
	}); err != nil {
		t.Fatalf("first registration: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	result, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan F",
		Email: "f@corp.example",
	})
	if err != nil {
		t.Fatalf("expected the expired slot to be reclaimed, got: %v", err)
	}
	if result.Availability.Registered != 1 {
		t.Errorf("registered = %d, want 1 (the expired row must not count)", result.Availability.Registered)
	}
}

// TestRegisterHandler_SweepsExpiredRowStatusOnRegister verifies the opportunistic
// sweep actually persists status="expired" on the stale row (not just excludes
// it from counts) — so anyone reading raw data sees accurate state without a
// cron job or queue.
func TestRegisterHandler_SweepsExpiredRowStatusOnRegister(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 2, 30*time.Millisecond)

	first, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan E",
		Email: "e@corp.example",
	})
	if err != nil {
		t.Fatalf("first registration: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan F",
		Email: "f@corp.example",
	}); err != nil {
		t.Fatalf("second registration: %v", err)
	}

	stale, err := repo.FindByCode(context.Background(), first.RegisCode)
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if stale.Status != domain.StatusExpired {
		t.Errorf("status = %q, want %q (sweep should have persisted it)", stale.Status, domain.StatusExpired)
	}
}

// TestRegisterHandler_DuplicateEmailAllowedAfterExpiry verifies that an email
// whose only registration expired unverified is free to register again.
func TestRegisterHandler_DuplicateEmailAllowedAfterExpiry(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 30, 30*time.Millisecond)

	if _, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan G",
		Email: "g@corp.example",
	}); err != nil {
		t.Fatalf("first registration: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan G",
		Email: "g@corp.example",
	}); err != nil {
		t.Fatalf("expected re-registration after expiry to succeed, got: %v", err)
	}
}

// TestRegisterHandler_ConcurrentLastSlot_OnlyOneSucceeds reproduces the brief's
// scenario: two employees grab the last slot at nearly the same time — exactly
// one registration must succeed and the other must be rejected with QUOTA_FULL.
func TestRegisterHandler_ConcurrentLastSlot_OnlyOneSucceeds(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 1, longTTL)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	emails := []string{"first@corp.example", "second@corp.example"}

	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := handler.Handle(context.Background(), application.RegisterCommand{
				Name:  "Concurrent",
				Email: emails[i],
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	successes, quotaFull := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, application.ErrQuotaFull):
			quotaFull++
		}
	}

	if successes != 1 || quotaFull != 1 {
		t.Fatalf("successes=%d quotaFull=%d errs=%v, want exactly one of each", successes, quotaFull, errs)
	}

	if count := repo.count(); count != 1 {
		t.Fatalf("final count = %d, want 1 (quota must never be exceeded)", count)
	}
}

func seedEmail(i int) string {
	return fmt.Sprintf("seed%d@corp.example", i)
}
