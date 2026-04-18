package database

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens the SQLite file at dbPath and returns a ready GORM instance.
// We also set two SQLite pragmas that matter for a POS system:
//   - WAL mode: allows reads while a write is happening (important for multi-user LAN use)
//   - foreign_keys ON: enforces your ON DELETE CASCADE rules from the schema
func Connect(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	// Get the underlying sql.DB so we can run raw PRAGMA statements
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if _, err = sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: could not set WAL mode: %v", err)
	}

	if _, err = sqlDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: could not enable foreign keys: %v", err)
	}

	log.Println("Database connected:", dbPath)
	return db, nil
}