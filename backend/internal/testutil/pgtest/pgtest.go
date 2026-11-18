// Package pgtest gives tests real, isolated, migrated Postgres databases.
//
// One Postgres container is started per test package (lazily, on first use)
// and stopped when the package's tests finish. The goose migrations are
// applied once to a template database; every call to New or DSN then clones
// that template into a brand-new database for the calling test and drops it
// when the test ends. Tests therefore always start from a clean, fully
// migrated schema and may run in parallel.
//
// Wire it up once per package:
//
//	func TestMain(m *testing.M) { pgtest.Main(m) }
//
// Only Docker (or a Docker-compatible engine) is required. Tests using this
// package are skipped under `go test -short`.
package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	"github.com/bayu-aditya/data-platform/server/internal/database"
)

// Image is the Postgres image the container runs.
const Image = "postgres:17-alpine"

const templateDB = "pgtest_template"

var (
	mainCalled atomic.Bool

	startOnce sync.Once
	cluster   *pgCluster
	startErr  error

	dbSeq atomic.Int64
)

// Main runs the package's tests and then stops the container, if one was
// started. Call it from TestMain.
func Main(m *testing.M) {
	mainCalled.Store(true)
	code := m.Run()
	if cluster != nil {
		if err := cluster.close(); err != nil {
			fmt.Fprintf(os.Stderr, "pgtest: stop postgres: %v\n", err)
		}
	}
	os.Exit(code)
}

// New returns a GORM handle to a fresh, fully migrated database that is
// dropped when t ends.
func New(t testing.TB) *gorm.DB {
	t.Helper()

	db, err := database.Open(t.Context(), DSN(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	// Registered after DSN's cleanup, so it runs first: close before drop.
	t.Cleanup(func() { _ = database.Close(db) })
	return db
}

// DSN returns the connection string of a fresh, fully migrated database
// that is dropped when t ends.
func DSN(t testing.TB) string {
	t.Helper()
	return newDatabase(t, templateDB)
}

// EmptyDSN returns the connection string of a fresh database with no
// migrations applied, for testing the migrations themselves. It is dropped
// when t ends.
func EmptyDSN(t testing.TB) string {
	t.Helper()
	return newDatabase(t, "template0")
}

func newDatabase(t testing.TB, template string) string {
	t.Helper()

	if testing.Short() {
		t.Skip("pgtest: skipping database test in -short mode")
	}
	if !mainCalled.Load() {
		t.Fatal("pgtest: call pgtest.Main(m) from this package's TestMain")
	}

	startOnce.Do(func() { cluster, startErr = startCluster() })
	if startErr != nil {
		t.Fatalf("pgtest: %v", startErr)
	}

	name := fmt.Sprintf("pgtest_%d_%d", os.Getpid(), dbSeq.Add(1))
	if err := cluster.createDatabase(t.Context(), name, template); err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := cluster.dropDatabase(ctx, name); err != nil {
			t.Errorf("pgtest: %v", err)
		}
	})

	return cluster.dsn(name)
}

// pgCluster is the running container plus an admin connection used to
// create and drop per-test databases.
type pgCluster struct {
	container *tcpostgres.PostgresContainer
	baseURL   *url.URL
	admin     *sql.DB

	// Postgres refuses to clone a template while another session (such as
	// a concurrent CREATE DATABASE) is using it, so clones are serialized.
	createMu sync.Mutex
}

// randomPassword returns a throwaway password for the test container, so no
// credential is hard-coded in the source.
func randomPassword() string {
	return rand.Text()
}

func startCluster() (*pgCluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx, Image,
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword(randomPassword()),
		tcpostgres.BasicWaitStrategies(),
		// Durability is irrelevant for throwaway test data.
		testcontainers.WithCmdArgs("-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off"),
	)
	if err != nil {
		// Run may return a created-but-unstarted container alongside the
		// error; TerminateContainer is nil-safe.
		_ = testcontainers.TerminateContainer(container)
		return nil, fmt.Errorf("start postgres container (is Docker running?): %w", err)
	}

	c := &pgCluster{container: container}
	if err := c.init(ctx); err != nil {
		_ = c.close()
		return nil, err
	}
	return c, nil
}

func (c *pgCluster) init(ctx context.Context) error {
	baseDSN, err := c.container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("postgres connection string: %w", err)
	}
	if c.baseURL, err = url.Parse(baseDSN); err != nil {
		return fmt.Errorf("parse postgres connection string: %w", err)
	}

	c.admin, err = sql.Open("pgx", baseDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}

	if err := c.createDatabase(ctx, templateDB, "template0"); err != nil {
		return err
	}
	return c.migrate(ctx, templateDB)
}

func (c *pgCluster) migrate(ctx context.Context, name string) error {
	db, err := sql.Open("pgx", c.dsn(name))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", name, err)
	}
	// Closing matters: the template cannot be cloned while connected to.
	defer db.Close()

	migrator, err := database.NewMigrator(db)
	if err != nil {
		return err
	}
	if _, err := migrator.Up(ctx); err != nil {
		return fmt.Errorf("migrate %s: %w", name, err)
	}
	return nil
}

func (c *pgCluster) createDatabase(ctx context.Context, name, template string) error {
	c.createMu.Lock()
	defer c.createMu.Unlock()

	stmt := fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s",
		pgx.Identifier{name}.Sanitize(), pgx.Identifier{template}.Sanitize())
	if _, err := c.admin.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	return nil
}

func (c *pgCluster) dropDatabase(ctx context.Context, name string) error {
	stmt := fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", pgx.Identifier{name}.Sanitize())
	if _, err := c.admin.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("drop database %s: %w", name, err)
	}
	return nil
}

// dsn points the container's base DSN (a postgres:// URL) at another
// database on the same server.
func (c *pgCluster) dsn(name string) string {
	u := *c.baseURL
	u.Path = "/" + name
	return u.String()
}

func (c *pgCluster) close() error {
	if c.admin != nil {
		_ = c.admin.Close()
	}
	return testcontainers.TerminateContainer(c.container)
}
