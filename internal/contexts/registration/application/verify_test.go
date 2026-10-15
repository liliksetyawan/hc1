package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/domain"
)

func TestVerifyHandler_Success(t *testing.T) {
	repo := newFakeRepository()
	registerHandler := application.NewRegisterHandler(repo, 30, longTTL)
	verifyHandler := application.NewVerifyHandler(repo)

	reg, err := registerHandler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan A",
		Email: "a@corp.example",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	result, err := verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: reg.RegisCode})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("status = %q, want success", result.Status)
	}
	if result.RegistrationID != reg.RegistrationID {
		t.Errorf("registrationId = %q, want %q", result.RegistrationID, reg.RegistrationID)
	}
}

func TestVerifyHandler_IdempotentOnAlreadySuccess(t *testing.T) {
	repo := newFakeRepository()
	registerHandler := application.NewRegisterHandler(repo, 30, longTTL)
	verifyHandler := application.NewVerifyHandler(repo)

	reg, _ := registerHandler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan A",
		Email: "a@corp.example",
	})
	if _, err := verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: reg.RegisCode}); err != nil {
		t.Fatalf("first verify: %v", err)
	}

	result, err := verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: reg.RegisCode})
	if err != nil {
		t.Fatalf("second verify should be idempotent, got error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("status = %q, want success", result.Status)
	}
}

func TestVerifyHandler_Expired(t *testing.T) {
	repo := newFakeRepository()
	registerHandler := application.NewRegisterHandler(repo, 30, 30*time.Millisecond)
	verifyHandler := application.NewVerifyHandler(repo)

	reg, err := registerHandler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan A",
		Email: "a@corp.example",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	_, err = verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: reg.RegisCode})
	if !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}

	// Verifying again should consistently report expired (status was persisted).
	_, err = verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: reg.RegisCode})
	if !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("second verify err = %v, want ErrExpired", err)
	}
}

func TestVerifyHandler_NotFound(t *testing.T) {
	repo := newFakeRepository()
	verifyHandler := application.NewVerifyHandler(repo)

	_, err := verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: "does-not-exist"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestVerifyHandler_EmptyCode(t *testing.T) {
	repo := newFakeRepository()
	verifyHandler := application.NewVerifyHandler(repo)

	_, err := verifyHandler.Handle(context.Background(), application.VerifyCommand{Code: "   "})
	if !errors.Is(err, application.ErrInvalidCode) {
		t.Fatalf("err = %v, want ErrInvalidCode", err)
	}
}

func TestVerifyHandler_PropagatesRepositoryErrors(t *testing.T) {
	t.Run("FindByCode", func(t *testing.T) {
		handler := application.NewVerifyHandler(&erroringRepository{failOn: "FindByCode"})
		_, err := handler.Handle(context.Background(), application.VerifyCommand{Code: "any"})
		if !errors.Is(err, errRepo) {
			t.Fatalf("err = %v, want errRepo", err)
		}
	})
}
