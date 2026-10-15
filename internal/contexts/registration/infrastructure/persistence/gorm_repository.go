package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"salt/hackethon/internal/contexts/registration/domain"
)

const TableName = "workshop_registrations"

// advisoryLockKey is an arbitrary constant identifying the quota-sensitive
// critical section. One key is enough: registration volume for a single
// workshop is low, and pg_advisory_xact_lock auto-releases at transaction end.
const advisoryLockKey = int64(851274)

type model struct {
	ID         int64      `gorm:"column:id;primaryKey"`
	Name       string     `gorm:"column:name"`
	Email      string     `gorm:"column:email"`
	Status     string     `gorm:"column:status"`
	RegisCode  string     `gorm:"column:regis_code"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	ExpiresAt  time.Time  `gorm:"column:expires_at"`
	VerifiedAt *time.Time `gorm:"column:verified_at"`
}

func (model) TableName() string { return TableName }

func toDomain(m *model) *domain.Registration {
	return &domain.Registration{
		ID:         m.ID,
		Name:       m.Name,
		Email:      m.Email,
		Status:     domain.Status(m.Status),
		RegisCode:  m.RegisCode,
		CreatedAt:  m.CreatedAt,
		ExpiresAt:  m.ExpiresAt,
		VerifiedAt: m.VerifiedAt,
	}
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		return fn(&Repository{db: txDB})
	})
}

func (r *Repository) Lock(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(?)", advisoryLockKey).Error
}

// activeCondition matches rows that currently occupy a quota slot: confirmed
// registrations, or pending ones whose TTL hasn't passed yet. A pending row
// past its expires_at is treated as inactive even though its stored status
// column may still literally say "pending" until something writes to it.
const activeCondition = "status = ? OR (status = ? AND expires_at > ?)"

func (r *Repository) ExpirePending(ctx context.Context, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model{}).
		Where("status = ? AND expires_at <= ?", domain.StatusPending, now).
		Update("status", string(domain.StatusExpired)).Error
}

func (r *Repository) CountActive(ctx context.Context, now time.Time) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model{}).
		Where(activeCondition, domain.StatusSuccess, domain.StatusPending, now).
		Count(&count).Error
	return int(count), err
}

func (r *Repository) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model{}).
		Where("email = ?", email).
		Where(activeCondition, domain.StatusSuccess, domain.StatusPending, now).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) Create(ctx context.Context, reg *domain.Registration) error {
	m := &model{
		Name:      reg.Name,
		Email:     reg.Email,
		Status:    string(reg.Status),
		RegisCode: reg.RegisCode,
		CreatedAt: reg.CreatedAt,
		ExpiresAt: reg.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	reg.ID = m.ID
	return nil
}

func (r *Repository) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	var m model
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("regis_code = ?", code).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

func (r *Repository) Save(ctx context.Context, reg *domain.Registration) error {
	return r.db.WithContext(ctx).Model(&model{}).Where("id = ?", reg.ID).Updates(map[string]any{
		"status":      string(reg.Status),
		"verified_at": reg.VerifiedAt,
	}).Error
}

var _ domain.Repository = (*Repository)(nil)
