// Command server runs the data-platform RPC server, or manages the metadata
// database schema.
//
//	server [serve]                  run the RPC server (default)
//	server migrate up|down|status   apply, roll back one, or list migrations
//
// Configuration comes from the environment; see internal/config.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bayu-aditya/data-platform/server/internal/config"
)

const usage = `usage:
  server [serve]                  run the RPC server (default)
  server migrate up|down|status   apply all, roll back one, or list migrations`

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Stop on SIGINT/SIGTERM so container orchestrators can drain us.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Getenv, logger)
	stop()

	if err != nil {
		logger.Error("command failed", slog.Any("error", err))
		os.Exit(1)
	}
}

// run dispatches to the subcommand named by args and returns when it is
// done: for serve, after ctx is cancelled and the server has drained.
func run(ctx context.Context, args []string, getenv func(string) string, logger *slog.Logger) error {
	cmd, rest := "serve", args
	if len(args) > 0 {
		cmd, rest = args[0], args[1:]
	}

	switch cmd {
	case "serve":
		if len(rest) != 0 {
			return fmt.Errorf("serve takes no arguments\n%s", usage)
		}
		cfg, err := config.Load(getenv)
		if err != nil {
			return err
		}
		return serve(ctx, cfg, logger)

	case "migrate":
		var action migrateAction
		if len(rest) == 1 {
			action = migrateActions[rest[0]]
		}
		if action == nil {
			return fmt.Errorf("migrate needs exactly one of up, down, status\n%s", usage)
		}
		cfg, err := config.Load(getenv)
		if err != nil {
			return err
		}
		return migrate(ctx, cfg, action, logger)

	default:
		return fmt.Errorf("unknown command %q\n%s", cmd, usage)
	}
}
