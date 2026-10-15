package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/domain"
)

var errRepo = errors.New("repository unavailable")

// erroringRepository simulates infrastructure failures (e.g. DB connection lost)
// so the application layer's error propagation can be verified without a real DB.
type erroringRepository struct {
	failOn string
}

func (r *erroringRepository) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	return fn(r)
}

func (r *erroringRepository) Lock(ctx context.Context) error {
	if r.failOn == "Lock" {
		return errRepo
	}
	return nil
}

func (r *erroringRepository) ExpirePending(ctx context.Context, now time.Time) error {
	if r.failOn == "ExpirePending" {
		return errRepo
	}
	return nil
}

func (r *erroringRepository) CountActive(ctx context.Context, now time.Time) (int, error) {
	if r.failOn == "CountActive" {
		return 0, errRepo
	}
	return 0, nil
}

func (r *erroringRepository) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	if r.failOn == "ExistsActiveByEmail" {
		return false, errRepo
	}
	return false, nil
}

func (r *erroringRepository) Create(ctx context.Context, reg *domain.Registration) error {
	if r.failOn == "Create" {
		return errRepo
	}
	reg.ID = 1
	return nil
}

func (r *erroringRepository) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	if r.failOn == "FindByCode" {
		return nil, errRepo
	}
	return nil, domain.ErrNotFound
}

func (r *erroringRepository) Save(ctx context.Context, reg *domain.Registration) error {
	if r.failOn == "Save" {
		return errRepo
	}
	return nil
}

var _ domain.Repository = (*erroringRepository)(nil)

func TestRegisterHandler_PropagatesRepositoryErrors(t *testing.T) {
	for _, failOn := range []string{"Lock", "ExpirePending", "ExistsActiveByEmail", "CountActive", "Create"} {
		t.Run(failOn, func(t *testing.T) {
			handler := application.NewRegisterHandler(&erroringRepository{failOn: failOn}, 30, longTTL)
			_, err := handler.Handle(context.Background(), application.RegisterCommand{
				Name:  "Karyawan A",
				Email: "a@corp.example",
			})
			if !errors.Is(err, errRepo) {
				t.Fatalf("err = %v, want errRepo", err)
			}
		})
	}
}

func TestAvailabilityHandler_PropagatesRepositoryError(t *testing.T) {
	for _, failOn := range []string{"ExpirePending", "CountActive"} {
		t.Run(failOn, func(t *testing.T) {
			handler := application.NewAvailabilityHandler(&erroringRepository{failOn: failOn}, 30)
			_, err := handler.Handle(context.Background())
			if !errors.Is(err, errRepo) {
				t.Fatalf("err = %v, want errRepo", err)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &application.ValidationError{Fields: []application.FieldError{{Field: "name", Message: "required"}}}
	if err.Error() != "validation error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "validation error")
	}
}

func TestRegisterHandler_ValidationError_EmptyEmail(t *testing.T) {
	repo := newFakeRepository()
	handler := application.NewRegisterHandler(repo, 30, longTTL)

	_, err := handler.Handle(context.Background(), application.RegisterCommand{
		Name:  "Karyawan A",
		Email: "   ",
	})

	var valErr *application.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if len(valErr.Fields) != 1 || valErr.Fields[0].Field != "email" {
		t.Errorf("fields = %+v, want single email error", valErr.Fields)
	}
}
