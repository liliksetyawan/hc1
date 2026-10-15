package database_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"salt/hackethon/internal/platform/database"
)

func newMockGormDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return gdb, mock
}

func TestMigrate_FileNotFound(t *testing.T) {
	gdb, _ := newMockGormDB(t)

	err := database.Migrate(gdb, filepath.Join(t.TempDir(), "does-not-exist.sql"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
}

func TestMigrate_Success(t *testing.T) {
	gdb, mock := newMockGormDB(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "0001_test.sql")
	sql := "CREATE TABLE IF NOT EXISTS workshop_registrations (id BIGSERIAL PRIMARY KEY);"
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatalf("write migration fixture: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(sql)).WillReturnResult(sqlmock.NewResult(0, 0))

	if err := database.Migrate(gdb, path); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigrate_ExecError(t *testing.T) {
	gdb, mock := newMockGormDB(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "0001_test.sql")
	sql := "CREATE TABLE broken syntax;"
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatalf("write migration fixture: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(sql)).WillReturnError(errors.New("syntax error"))

	if err := database.Migrate(gdb, path); err == nil {
		t.Fatal("expected error, got nil")
	}
}
