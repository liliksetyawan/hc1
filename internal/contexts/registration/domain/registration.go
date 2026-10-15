package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSuccess Status = "success"
	StatusExpired Status = "expired"
)

var (
	ErrNotFound = errors.New("registration not found")
	ErrExpired  = errors.New("registration expired")
)

// Registration is a reserved or confirmed seat in the workshop. A pending
// registration holds its seat only until ExpiresAt; if it is never verified,
// IsActive stops counting it without any row needing to change.
type Registration struct {
	ID         int64
	Name       string
	Email      string
	Status     Status
	RegisCode  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	VerifiedAt *time.Time
}

func NewRegistration(name, email, regisCode string, now time.Time, ttl time.Duration) *Registration {
	return &Registration{
		Name:      name,
		Email:     email,
		Status:    StatusPending,
		RegisCode: regisCode,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

// NormalizeEmail trims and lowercases an email so duplicate checks and storage
// are consistent regardless of incidental whitespace or casing.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsActive reports whether this registration currently occupies a quota slot.
func (r *Registration) IsActive(now time.Time) bool {
	switch r.Status {
	case StatusSuccess:
		return true
	case StatusPending:
		return now.Before(r.ExpiresAt)
	default:
		return false
	}
}

// Verify transitions a pending registration to success if it is still within
// its TTL. If the TTL has passed, it transitions to expired instead and
// returns ErrExpired — callers must still persist the entity in that case so
// the expired status is recorded. Re-verifying an already-successful
// registration is a no-op (idempotent links are safer for a browser-clicked
// confirmation URL).
func (r *Registration) Verify(now time.Time) error {
	switch r.Status {
	case StatusSuccess:
		return nil
	case StatusExpired:
		return ErrExpired
	default:
		if !now.Before(r.ExpiresAt) {
			r.Status = StatusExpired
			return ErrExpired
		}
		r.Status = StatusSuccess
		r.VerifiedAt = &now
		return nil
	}
}

// Repository is implemented by the infrastructure layer. WithTransaction and Lock
// exist so the application layer can serialize the read-count-then-insert sequence
// that enforces the quota under concurrent registrations.
type Repository interface {
	WithTransaction(ctx context.Context, fn func(tx Repository) error) error
	Lock(ctx context.Context) error
	// ExpirePending flips any pending row past its TTL to expired. Counting
	// (CountActive/ExistsActiveByEmail) never depends on this having run —
	// it's purely so the stored status column reflects reality for anyone
	// reading raw data, without needing a cron job or queue to keep it fresh.
	ExpirePending(ctx context.Context, now time.Time) error
	CountActive(ctx context.Context, now time.Time) (int, error)
	ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error)
	Create(ctx context.Context, r *Registration) error
	FindByCode(ctx context.Context, code string) (*Registration, error)
	Save(ctx context.Context, r *Registration) error
}
