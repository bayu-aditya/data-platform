package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"

	"github.com/bayu-aditya/data-platform/server/internal/config"
	"github.com/bayu-aditya/data-platform/server/internal/database"
)

// migrateAction runs against a connected goose provider and logs what it
// did.
type migrateAction func(context.Context, *goose.Provider, *slog.Logger) error

// migrateActions are the `migrate` subcommand's arguments.
var migrateActions = map[string]migrateAction{
	"up":     migrateUp,
	"down":   migrateDown,
	"status": migrateStatus,
}

// migrate applies, rolls back, or lists the embedded migrations, then
// returns. It is meant to run as a one-off job before rolling out serve.
func migrate(ctx context.Context, cfg config.Config, action migrateAction, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(db); err != nil {
			logger.Warn("close metadata database", slog.Any("error", err))
		}
	}()

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	migrator, err := database.NewMigrator(sqlDB)
	if err != nil {
		return err
	}
	return action(ctx, migrator, logger)
}

func migrateUp(ctx context.Context, migrator *goose.Provider, logger *slog.Logger) error {
	results, err := migrator.Up(ctx)
	for _, r := range results {
		logResult(logger, r)
	}
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(results) == 0 {
		logger.Info("no pending migrations")
	}
	return nil
}

func migrateDown(ctx context.Context, migrator *goose.Provider, logger *slog.Logger) error {
	result, err := migrator.Down(ctx)
	if result != nil {
		logResult(logger, result)
	}
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func migrateStatus(ctx context.Context, migrator *goose.Provider, logger *slog.Logger) error {
	statuses, err := migrator.Status(ctx)
	if err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	for _, s := range statuses {
		attrs := []any{
			slog.Int64("version", s.Source.Version),
			slog.String("file", s.Source.Path),
			slog.String("state", string(s.State)),
		}
		if !s.AppliedAt.IsZero() {
			attrs = append(attrs, slog.Time("applied_at", s.AppliedAt))
		}
		logger.Info("migration", attrs...)
	}
	return nil
}

func logResult(logger *slog.Logger, r *goose.MigrationResult) {
	attrs := []any{
		slog.String("direction", r.Direction),
		slog.Int64("version", r.Source.Version),
		slog.String("file", r.Source.Path),
		slog.Duration("duration", r.Duration),
	}
	if r.Error != nil {
		logger.Error("migration failed", append(attrs, slog.Any("error", r.Error))...)
		return
	}
	logger.Info("migration applied", attrs...)
}
