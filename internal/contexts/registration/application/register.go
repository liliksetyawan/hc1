package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"salt/hackethon/internal/contexts/registration/domain"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrQuotaFull              = errors.New("quota full")
)

type FieldError struct {
	Field   string
	Message string
}

type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	return "validation error"
}

type RegisterCommand struct {
	Name  string
	Email string
}

type Availability struct {
	Total      int
	Registered int
	Remaining  int
}

type RegisterResult struct {
	RegistrationID string
	Name           string
	Email          string
	Status         string
	RegisCode      string
	Availability   Availability
}

type RegisterHandler struct {
	repo       domain.Repository
	quotaTotal int
	ttl        time.Duration
}

func NewRegisterHandler(repo domain.Repository, quotaTotal int, ttl time.Duration) *RegisterHandler {
	return &RegisterHandler{repo: repo, quotaTotal: quotaTotal, ttl: ttl}
}

func (h *RegisterHandler) Handle(ctx context.Context, cmd RegisterCommand) (*RegisterResult, error) {
	name := strings.TrimSpace(cmd.Name)
	email := domain.NormalizeEmail(cmd.Email)

	if fields := validateRegistration(name, email); len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}

	var result *RegisterResult
	err := h.repo.WithTransaction(ctx, func(tx domain.Repository) error {
		if err := tx.Lock(ctx); err != nil {
			return err
		}

		now := time.Now()

		if err := tx.ExpirePending(ctx, now); err != nil {
			return err
		}

		exists, err := tx.ExistsActiveByEmail(ctx, email, now)
		if err != nil {
			return err
		}
		if exists {
			return ErrEmailAlreadyRegistered
		}

		countBefore, err := tx.CountActive(ctx, now)
		if err != nil {
			return err
		}
		if countBefore >= h.quotaTotal {
			return ErrQuotaFull
		}

		code, err := generateRegisCode()
		if err != nil {
			return err
		}

		reg := domain.NewRegistration(name, email, code, now, h.ttl)
		if err := tx.Create(ctx, reg); err != nil {
			return err
		}

		registered := countBefore + 1
		result = &RegisterResult{
			RegistrationID: formatRegistrationID(reg.ID),
			Name:           reg.Name,
			Email:          reg.Email,
			Status:         string(reg.Status),
			RegisCode:      reg.RegisCode,
			Availability: Availability{
				Total:      h.quotaTotal,
				Registered: registered,
				Remaining:  h.quotaTotal - registered,
			},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateRegistration(name, email string) []FieldError {
	var fields []FieldError

	if name == "" {
		fields = append(fields, FieldError{Field: "name", Message: "Nama wajib diisi."})
	}

	if email == "" {
		fields = append(fields, FieldError{Field: "email", Message: "Format email tidak valid."})
	} else if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		fields = append(fields, FieldError{Field: "email", Message: "Format email tidak valid."})
	}

	return fields
}

// formatRegistrationID is derived from the monotonic DB primary key, not
// from the active-count-at-creation-time: since pending registrations can
// expire and free their slot, "active count" is not monotonic and reusing it
// here could assign the same REG-XXXX to two different people over time.
func formatRegistrationID(id int64) string {
	return fmt.Sprintf("REG-%04d", id)
}

// generateRegisCode produces a 32-hex-char (128-bit) random token. The
// collision probability is negligible, so unlike the quota/duplicate checks
// this isn't re-verified under the advisory lock before insert.
func generateRegisCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
