package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bayu-aditya/data-platform/server/internal/config"
	"github.com/bayu-aditya/data-platform/server/internal/database"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/server"
	"github.com/bayu-aditya/data-platform/server/internal/usecase"
)

// shutdownTimeout bounds how long in-flight requests get to finish after a
// termination signal.
const shutdownTimeout = 15 * time.Second

// serve runs the RPC server until ctx is cancelled. It connects to the
// metadata database first and refuses to start if that fails. It never runs
// migrations: the schema is owned by the `migrate` subcommand.
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(db); err != nil {
			logger.Warn("close metadata database", slog.Any("error", err))
		}
	}()

	srv := server.New(server.Config{
		Addr:         cfg.GRPCAddr,
		Environments: usecase.NewEnvironmentUsecase(repository.NewEnvironmentRepository(db)),
	})

	errs := make(chan error, 1)
	go func() {
		logger.Info("rpc server listening", slog.String("addr", srv.Addr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return <-errs
}
