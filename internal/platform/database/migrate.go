package database

import (
	"os"

	"gorm.io/gorm"
)

// Migrate applies the SQL file at path. Statements must be idempotent
// (CREATE TABLE IF NOT EXISTS, ...) since this runs on every startup.
func Migrate(db *gorm.DB, path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return db.Exec(string(sqlBytes)).Error
}
