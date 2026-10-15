package application_test

import (
	"context"
	"sync"
	"time"

	"salt/hackethon/internal/contexts/registration/domain"
)

// fakeRepository is an in-memory stand-in for the GORM repository. lockMu plays
// the role of the Postgres advisory lock taken inside Lock(): it is held for the
// entire WithTransaction call, so concurrent Handle() calls are serialized around
// it exactly like concurrent transactions would be around pg_advisory_xact_lock.
type fakeRepository struct {
	lockMu sync.Mutex
	dataMu sync.Mutex
	byID   map[int64]*domain.Registration
	byCode map[string]int64
	nextID int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		byID:   make(map[int64]*domain.Registration),
		byCode: make(map[string]int64),
	}
}

func (f *fakeRepository) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	tx := &fakeTx{repo: f}
	err := fn(tx)
	if tx.locked {
		f.lockMu.Unlock()
	}
	return err
}

func (f *fakeRepository) Lock(ctx context.Context) error {
	f.lockMu.Lock()
	return nil
}

func (f *fakeRepository) ExpirePending(ctx context.Context, now time.Time) error {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	for _, r := range f.byID {
		if r.Status == domain.StatusPending && !now.Before(r.ExpiresAt) {
			r.Status = domain.StatusExpired
		}
	}
	return nil
}

func (f *fakeRepository) CountActive(ctx context.Context, now time.Time) (int, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	count := 0
	for _, r := range f.byID {
		if r.IsActive(now) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepository) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	for _, r := range f.byID {
		if r.Email == email && r.IsActive(now) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRepository) Create(ctx context.Context, reg *domain.Registration) error {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	f.nextID++
	reg.ID = f.nextID
	cp := *reg
	f.byID[reg.ID] = &cp
	f.byCode[reg.RegisCode] = reg.ID
	return nil
}

func (f *fakeRepository) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	id, ok := f.byCode[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *f.byID[id]
	return &cp, nil
}

func (f *fakeRepository) Save(ctx context.Context, reg *domain.Registration) error {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	cp := *reg
	f.byID[reg.ID] = &cp
	return nil
}

func (f *fakeRepository) count() int {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	return len(f.byID)
}

// fakeTx is the handle passed into the WithTransaction callback. Lock() acquires
// the outer repository's lockMu and records that it did so, so WithTransaction
// knows whether to release it afterwards (Lock is optional — AvailabilityHandler
// never calls it, matching a plain read with no advisory lock).
type fakeTx struct {
	repo   *fakeRepository
	locked bool
}

func (t *fakeTx) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	return fn(t)
}

func (t *fakeTx) Lock(ctx context.Context) error {
	err := t.repo.Lock(ctx)
	t.locked = true
	return err
}

func (t *fakeTx) ExpirePending(ctx context.Context, now time.Time) error {
	return t.repo.ExpirePending(ctx, now)
}

func (t *fakeTx) CountActive(ctx context.Context, now time.Time) (int, error) {
	return t.repo.CountActive(ctx, now)
}

func (t *fakeTx) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	return t.repo.ExistsActiveByEmail(ctx, email, now)
}

func (t *fakeTx) Create(ctx context.Context, reg *domain.Registration) error {
	return t.repo.Create(ctx, reg)
}

func (t *fakeTx) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	return t.repo.FindByCode(ctx, code)
}

func (t *fakeTx) Save(ctx context.Context, reg *domain.Registration) error {
	return t.repo.Save(ctx, reg)
}

var (
	_ domain.Repository = (*fakeRepository)(nil)
	_ domain.Repository = (*fakeTx)(nil)
)
