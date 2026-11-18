// Package database connects to the metadata Postgres database and owns its
// schema migrations.
//
// The schema is managed only by goose migrations (see Migrator), run by the
// `migrate` subcommand. Nothing in this codebase may call GORM's AutoMigrate.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// connectTimeout bounds the startup connectivity check, so an unreachable
// database fails the process quickly instead of hanging.
const connectTimeout = 10 * time.Second

// Open connects GORM (on the pgx driver) to dsn and verifies the database is
// reachable before returning. Callers must Close the returned DB.
func Open(ctx context.Context, dsn string, log *slog.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Ping ourselves below, with a deadline and the caller's context.
		DisableAutomaticPing: true,
		// Report unique / foreign-key violations as gorm.ErrDuplicatedKey /
		// gorm.ErrForeignKeyViolated so repositories can classify them.
		TranslateError: true,
		Logger: logger.NewSlogLogger(log, logger.Config{
			LogLevel:                  logger.Warn,
			SlowThreshold:             time.Second,
			IgnoreRecordNotFoundError: true,
			// Never log bound values: they may hold secrets.
			ParameterizedQueries: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open metadata database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open metadata database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("metadata database unreachable: %w", err)
	}

	return db, nil
}

// Close releases the DB's connection pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
