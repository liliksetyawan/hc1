package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"salt/hackethon/internal/contexts/registration/domain"
)

var ErrInvalidCode = errors.New("invalid verification code")

type VerifyCommand struct {
	Code string
}

type VerifyResult struct {
	RegistrationID string
	Status         string
}

type VerifyHandler struct {
	repo domain.Repository
}

func NewVerifyHandler(repo domain.Repository) *VerifyHandler {
	return &VerifyHandler{repo: repo}
}

func (h *VerifyHandler) Handle(ctx context.Context, cmd VerifyCommand) (*VerifyResult, error) {
	code := strings.TrimSpace(cmd.Code)
	if code == "" {
		return nil, ErrInvalidCode
	}

	var result *VerifyResult
	err := h.repo.WithTransaction(ctx, func(tx domain.Repository) error {
		reg, err := tx.FindByCode(ctx, code)
		if err != nil {
			return err
		}

		verifyErr := reg.Verify(time.Now())
		if saveErr := tx.Save(ctx, reg); saveErr != nil {
			return saveErr
		}
		if verifyErr != nil {
			return verifyErr
		}

		result = &VerifyResult{RegistrationID: formatRegistrationID(reg.ID), Status: string(reg.Status)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
