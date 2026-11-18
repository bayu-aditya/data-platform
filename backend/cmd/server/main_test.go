package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // "pgx" database/sql driver

	"github.com/bayu-aditya/data-platform/server/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

var discard = slog.New(slog.DiscardHandler)

func envOf(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestRunRejectsBadArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"unknown command", []string{"frobnicate"}, `unknown command "frobnicate"`},
		{"serve with arguments", []string{"serve", "now"}, "serve takes no arguments"},
		{"migrate without action", []string{"migrate"}, "migrate needs exactly one of up, down, status"},
		{"migrate unknown action", []string{"migrate", "sideways"}, "migrate needs exactly one of up, down, status"},
		{"migrate extra arguments", []string{"migrate", "up", "now"}, "migrate needs exactly one of up, down, status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := run(t.Context(), tt.args, envOf(nil), discard)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{nil, {"serve"}, {"migrate", "up"}} {
		err := run(t.Context(), args, envOf(nil), discard)
		if err == nil || !strings.Contains(err.Error(), "DATABASE_URL: is required") {
			t.Errorf("run(%q): err = %v, want missing DATABASE_URL", args, err)
		}
	}
}

func TestServeFailsFastWhenDatabaseUnreachable(t *testing.T) {
	t.Parallel()

	env := envOf(map[string]string{
		"GRPC_ADDR":    freeAddr(t),
		"DATABASE_URL": "postgres://" + freeAddr(t) + "/app?sslmode=disable",
	})

	start := time.Now()
	err := run(t.Context(), nil, env, discard)

	if err == nil || !strings.Contains(err.Error(), "metadata database unreachable") {
		t.Fatalf("err = %v, want metadata database unreachable", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v to fail, want fail fast", elapsed)
	}
}

// TestServeStartsAndNeverMigrates boots the default (no subcommand) path
// against an empty database and checks the server comes up, drains on
// cancel, and leaves the schema untouched.
func TestServeStartsAndNeverMigrates(t *testing.T) {
	t.Parallel()

	dsn := pgtest.EmptyDSN(t)
	addr := freeAddr(t)
	env := envOf(map[string]string{"GRPC_ADDR": addr, "DATABASE_URL": dsn})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, env, discard) }()

	waitListening(t, addr, done)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}

	if hasGooseTable(t, dsn) {
		t.Error("serve created goose_db_version: it must never run migrations")
	}
}

func TestMigrateUpStatusDown(t *testing.T) {
	t.Parallel()

	dsn := pgtest.EmptyDSN(t)
	env := envOf(map[string]string{"DATABASE_URL": dsn})
	migrate := func(action string) {
		t.Helper()
		if err := run(t.Context(), []string{"migrate", action}, env, discard); err != nil {
			t.Fatalf("migrate %s: %v", action, err)
		}
	}

	migrate("status")
	if got := appliedVersions(t, dsn); len(got) != 0 {
		t.Fatalf("after status: applied = %v, want none", got)
	}

	migrate("up")
	applied := appliedVersions(t, dsn)
	if len(applied) == 0 || applied[0] != 1 {
		t.Fatalf("after up: applied = %v, want every migration starting at 1", applied)
	}

	migrate("up") // idempotent
	migrate("status")
	if got := appliedVersions(t, dsn); !slices.Equal(got, applied) {
		t.Fatalf("after second up: applied = %v, want %v", got, applied)
	}

	migrate("down") // rolls back only the latest migration
	if got := appliedVersions(t, dsn); !slices.Equal(got, applied[:len(applied)-1]) {
		t.Fatalf("after down: applied = %v, want %v", got, applied[:len(applied)-1])
	}
}

// appliedVersions lists the migration versions goose has recorded as
// applied, ignoring its version-0 bootstrap row.
func appliedVersions(t *testing.T, dsn string) []int64 {
	t.Helper()

	db := openSQL(t, dsn)
	if !hasGooseTable(t, dsn) {
		return nil
	}
	rows, err := db.QueryContext(t.Context(),
		`SELECT version_id FROM goose_db_version WHERE version_id > 0 ORDER BY version_id`)
	if err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return versions
}

func hasGooseTable(t *testing.T, dsn string) bool {
	t.Helper()

	var exists bool
	err := openSQL(t, dsn).QueryRowContext(t.Context(),
		`SELECT to_regclass('goose_db_version') IS NOT NULL`).Scan(&exists)
	if err != nil {
		t.Fatalf("check goose_db_version: %v", err)
	}
	return exists
}

func openSQL(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// freeAddr returns a loopback address with a port that was free a moment
// ago.
func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return addr
}

// waitListening polls addr until it accepts connections, failing early if
// the server exits first.
func waitListening(t *testing.T, addr string, exited <-chan error) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			t.Fatalf("server exited before listening: %v", err)
		default:
		}
		if conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s never started listening", addr)
}
