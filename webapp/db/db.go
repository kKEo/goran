// Package db opens the SQLite database and applies the schema.
package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkEo/g-mk8s/webapp/model"
)

// Options configures Open.
type Options struct {
	// Path is the SQLite file. Use ":memory:" for a throwaway database.
	Path   string
	LogSQL bool
}

// Open connects, limits the pool to one connection (SQLite has a single
// writer anyway, and it makes ":memory:" behave) and migrates the schema.
func Open(opts Options) (*gorm.DB, error) {
	if opts.Path == "" {
		opts.Path = "local.sqlite"
	}
	dsn := opts.Path
	if dsn != ":memory:" {
		dsn = fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", opts.Path)
	}
	level := logger.Warn
	if opts.LogSQL {
		level = logger.Info
	}
	gl := logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: true,
	})
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gl})
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", opts.Path, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(model.All()...); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}
