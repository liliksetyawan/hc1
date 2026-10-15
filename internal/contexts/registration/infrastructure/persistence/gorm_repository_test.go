package persistence

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"salt/hackethon/internal/contexts/registration/domain"
)

// newMockRepo wires a Repository against sqlmock instead of a real Postgres
// connection, so the SQL this package builds can be verified deterministically
// without a live DB. SkipDefaultTransaction simplifies the expected call
// sequence (no implicit BEGIN/COMMIT around single-statement calls) — it's a
// test-harness-only choice; production wiring in database.NewPostgres doesn't
// set it.
func newMockRepo(t *testing.T) (*Repository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return NewRepository(gdb), mock
}

func TestModel_TableName(t *testing.T) {
	if got := (model{}).TableName(); got != "workshop_registrations" {
		t.Errorf("TableName() = %q, want workshop_registrations", got)
	}
}

func TestToDomain(t *testing.T) {
	verifiedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := &model{
		ID: 7, Name: "Karyawan A", Email: "a@corp.example",
		Status: "success", RegisCode: "code123",
		CreatedAt: verifiedAt.Add(-time.Hour), ExpiresAt: verifiedAt.Add(time.Hour),
		VerifiedAt: &verifiedAt,
	}

	got := toDomain(m)

	want := &domain.Registration{
		ID: 7, Name: "Karyawan A", Email: "a@corp.example",
		Status: domain.StatusSuccess, RegisCode: "code123",
		CreatedAt: m.CreatedAt, ExpiresAt: m.ExpiresAt, VerifiedAt: &verifiedAt,
	}
	if got.ID != want.ID || got.Name != want.Name || got.Email != want.Email ||
		got.Status != want.Status || got.RegisCode != want.RegisCode ||
		!got.CreatedAt.Equal(want.CreatedAt) || !got.ExpiresAt.Equal(want.ExpiresAt) ||
		got.VerifiedAt == nil || !got.VerifiedAt.Equal(*want.VerifiedAt) {
		t.Errorf("toDomain() = %+v, want %+v", got, want)
	}
}

func TestNewRepository(t *testing.T) {
	repo, _ := newMockRepo(t)
	if repo == nil || repo.db == nil {
		t.Fatal("NewRepository returned a repo with nil db")
	}
}

func TestRepository_Lock(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock")).
		WithArgs(advisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := repo.Lock(context.Background()); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRepository_ExpirePending(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	mock.ExpectExec(`UPDATE "workshop_registrations" SET "status"`).
		WithArgs(string(domain.StatusExpired), string(domain.StatusPending), now).
		WillReturnResult(sqlmock.NewResult(0, 2))

	if err := repo.ExpirePending(context.Background(), now); err != nil {
		t.Fatalf("ExpirePending: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRepository_CountActive(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"count"}).AddRow(5)
	mock.ExpectQuery(`SELECT count\(\*\) FROM "workshop_registrations"`).
		WithArgs(string(domain.StatusSuccess), string(domain.StatusPending), now).
		WillReturnRows(rows)

	count, err := repo.CountActive(context.Background(), now)
	if err != nil {
		t.Fatalf("CountActive: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestRepository_CountActive_Error(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	mock.ExpectQuery(`SELECT count\(\*\) FROM "workshop_registrations"`).
		WillReturnError(errors.New("connection reset"))

	_, err := repo.CountActive(context.Background(), now)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRepository_ExistsActiveByEmail(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery(`SELECT count\(\*\) FROM "workshop_registrations" WHERE email = \$1 AND \(status = \$2 OR`).
		WithArgs("a@corp.example", string(domain.StatusSuccess), string(domain.StatusPending), now).
		WillReturnRows(rows)

	exists, err := repo.ExistsActiveByEmail(context.Background(), "a@corp.example", now)
	if err != nil {
		t.Fatalf("ExistsActiveByEmail: %v", err)
	}
	if !exists {
		t.Error("exists = false, want true")
	}
}

func TestRepository_ExistsActiveByEmail_NotFound(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
	mock.ExpectQuery(`SELECT count\(\*\) FROM "workshop_registrations"`).
		WillReturnRows(rows)

	exists, err := repo.ExistsActiveByEmail(context.Background(), "nobody@corp.example", now)
	if err != nil {
		t.Fatalf("ExistsActiveByEmail: %v", err)
	}
	if exists {
		t.Error("exists = true, want false")
	}
}

func TestRepository_Create(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	reg := &domain.Registration{
		Name: "Karyawan A", Email: "a@corp.example",
		Status: domain.StatusPending, RegisCode: "code123",
		CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	}

	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)
	mock.ExpectQuery(`INSERT INTO "workshop_registrations"`).
		WillReturnRows(rows)

	if err := repo.Create(context.Background(), reg); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if reg.ID != 42 {
		t.Errorf("reg.ID = %d, want 42 (must be populated from the insert)", reg.ID)
	}
}

func TestRepository_Create_Error(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	reg := &domain.Registration{Name: "A", Email: "a@corp.example", Status: domain.StatusPending, RegisCode: "c", CreatedAt: now, ExpiresAt: now}

	mock.ExpectQuery(`INSERT INTO "workshop_registrations"`).
		WillReturnError(errors.New("unique violation"))

	if err := repo.Create(context.Background(), reg); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRepository_FindByCode_Found(t *testing.T) {
	repo, mock := newMockRepo(t)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "email", "status", "regis_code", "created_at", "expires_at", "verified_at"}).
		AddRow(1, "Karyawan A", "a@corp.example", "pending", "code123", now, now.Add(time.Minute), nil)
	mock.ExpectQuery(`SELECT \* FROM "workshop_registrations" WHERE regis_code = \$1`).
		WithArgs("code123", 1).
		WillReturnRows(rows)

	reg, err := repo.FindByCode(context.Background(), "code123")
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if reg.ID != 1 || reg.RegisCode != "code123" {
		t.Errorf("reg = %+v, want id=1 regisCode=code123", reg)
	}
}

func TestRepository_FindByCode_NotFound(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`SELECT \* FROM "workshop_registrations" WHERE regis_code = \$1`).
		WithArgs("missing", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.FindByCode(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want domain.ErrNotFound", err)
	}
}

func TestRepository_FindByCode_Error(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`SELECT \* FROM "workshop_registrations" WHERE regis_code = \$1`).
		WillReturnError(errors.New("connection reset"))

	_, err := repo.FindByCode(context.Background(), "any")
	if err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want a generic (non-ErrNotFound) error", err)
	}
}

func TestRepository_Save(t *testing.T) {
	repo, mock := newMockRepo(t)
	verifiedAt := time.Now()
	reg := &domain.Registration{ID: 1, Status: domain.StatusSuccess, VerifiedAt: &verifiedAt}

	mock.ExpectExec(`UPDATE "workshop_registrations" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Save(context.Background(), reg); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestRepository_Save_Error(t *testing.T) {
	repo, mock := newMockRepo(t)
	reg := &domain.Registration{ID: 1, Status: domain.StatusExpired}

	mock.ExpectExec(`UPDATE "workshop_registrations" SET`).
		WillReturnError(errors.New("connection reset"))

	if err := repo.Save(context.Background(), reg); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRepository_WithTransaction_Commit(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock")).
		WithArgs(advisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	var gotTx domain.Repository
	err := repo.WithTransaction(context.Background(), func(tx domain.Repository) error {
		gotTx = tx
		return tx.Lock(context.Background())
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}
	if gotTx == nil {
		t.Fatal("callback never received a tx")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRepository_WithTransaction_Rollback(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	sentinelErr := errors.New("business rule violated")
	err := repo.WithTransaction(context.Background(), func(tx domain.Repository) error {
		return sentinelErr
	})
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("err = %v, want sentinelErr", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
