package database

import (
	"os"

	"gorm.io/gorm"
)

// Migrate applies the SQL file at path. Statements must be idempotent
// (CREATE TABLE IF NOT EXISTS, ...) since this runs on every startup.
func Migrate(db *gorm.DB, path string) error {
	sqlBytes, err := os.ReadFile(path) // #nosec G304 -- path is a fixed migration filename hardcoded in main.go, never derived from user/request input
	if err != nil {
		return err
	}
	return db.Exec(string(sqlBytes)).Error
}
