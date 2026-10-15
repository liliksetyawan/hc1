package application

import (
	"context"
	"time"

	"salt/hackethon/internal/contexts/registration/domain"
)

type AvailabilityHandler struct {
	repo       domain.Repository
	quotaTotal int
}

func NewAvailabilityHandler(repo domain.Repository, quotaTotal int) *AvailabilityHandler {
	return &AvailabilityHandler{repo: repo, quotaTotal: quotaTotal}
}

func (h *AvailabilityHandler) Handle(ctx context.Context) (*Availability, error) {
	now := time.Now()

	if err := h.repo.ExpirePending(ctx, now); err != nil {
		return nil, err
	}

	registered, err := h.repo.CountActive(ctx, now)
	if err != nil {
		return nil, err
	}
	remaining := h.quotaTotal - registered
	if remaining < 0 {
		remaining = 0
	}
	return &Availability{
		Total:      h.quotaTotal,
		Registered: registered,
		Remaining:  remaining,
	}, nil
}
