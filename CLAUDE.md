# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

This is a Go workspace (`go.work`, gitignored — not committed) joining two independent modules:

- `backend/` (module `github.com/bayu-aditya/data-platform/server`) — the Connect RPC server.
- `go-client/` (module `github.com/bayu-aditya/data-platform/go-client`) — a client SDK, currently just a module skeleton (no code yet).

Each module must also build standalone since `go.work`/`go.work.sum` aren't checked in.

## Commands

Run from `backend/`:

```bash
go build ./...
go test ./...                    # needs Docker: repository and cmd tests start Postgres via testcontainers
go test -short ./...             # skip every test that needs Docker
go test ./... -run TestName       # run a single test
buf lint                          # lint proto files under proto/
buf generate                      # regenerate gen/ from proto/
go tool mockery                   # regenerate all mocks (config: .mockery.yml)
```

`protoc-gen-go`, `protoc-gen-connect-go` and `mockery` are pinned via `tool` directives in `backend/go.mod`, so nothing needs a global install.

**Podman instead of Docker:** testcontainers' reaper container (Ryuk) fails under Podman ("unable to find network ... bridge"). Run tests with `TESTCONTAINERS_RYUK_DISABLED=true`; `pgtest.Main` stops its container itself, so nothing leaks unless the test process is killed.

### Server binary and subcommands

```bash
go run ./cmd/server                        # same as `serve`
go run ./cmd/server serve                  # run the RPC server
go run ./cmd/server migrate up             # apply all pending migrations, then exit
go run ./cmd/server migrate down           # roll back the latest migration, then exit
go run ./cmd/server migrate status         # log each migration's state, then exit
```

- `serve` connects to the metadata DB at startup and exits with an error if `DATABASE_URL` is missing or the DB is unreachable. It closes the DB after graceful shutdown (SIGINT/SIGTERM).
- `serve` **never** runs migrations. Schema changes ship via `migrate up`, run as a Kubernetes Job before the rollout.

### Configuration

All env vars are loaded in one place, `internal/config` (`config.Load`). An empty var counts as unset. Every invalid value is reported at once, and the process exits before doing anything else.

| Var | Default | Notes |
|---|---|---|
| `GRPC_ADDR` | `:8080` | Listen address |
| `DATABASE_URL` | — (required) | Metadata Postgres DSN |
| `QUERY_TIMEOUT` | `30s` | Positive Go duration |
| `QUERY_MAX_ROWS` | `100000` | Positive integer |

To add a var: add a field to `config.Config` and one line in `Load` using the matching `env` helper (`string`, `required`, `positiveDuration`, `positiveInt`; add a helper for a new type). Add a test case to `config_test.go`.

## Architecture

### Backend server

`backend/internal/server/server.go` builds an `http.Server` that speaks gRPC, gRPC-Web, and Connect simultaneously on one port over h2c (HTTP/2 without TLS) — a plain gRPC client can connect directly, no separate gateway. `server.New(cfg)` registers handlers on an `http.ServeMux` and returns a `*Server` with `ListenAndServe`/`Shutdown`. `backend/cmd/server` is the entrypoint: `main.go` dispatches subcommands, `serve.go` runs the server and `migrate.go` runs migrations.

Service health is reported via the standard `grpc.health.v1.Health` protocol (`connectrpc.com/grpchealth`), backed by a `grpchealth.StaticChecker` exposed through `Server.Health()`. A check with no service name always reports the process-wide status; a named check only succeeds for services explicitly passed to `grpchealth.NewStaticChecker`.

### Layering

```
handler (Connect RPC)  →  usecase (business rules)  →  repository (GORM, data access)
```

| Package | Depends on | Returns errors as |
|---|---|---|
| `internal/repository` | `*gorm.DB` | `domainerr` for expected outcomes (`gorm.ErrRecordNotFound` → `NotFound`, `gorm.ErrDuplicatedKey` → `AlreadyExists`, `gorm.ErrForeignKeyViolated` → `FailedPrecondition`); any other error unchanged |
| `internal/usecase` | repository **interfaces** only | `domainerr`; classify raw repository errors when the usecase knows what they mean (e.g. `Unavailable`), otherwise pass them on and they become `INTERNAL` |
| `internal/handler` | usecase **interfaces** only | `handler.ConnectError(ctx, err)`, always |

Rules:

- Each repository / usecase is an **exported interface** plus an **unexported implementation** returned by a `New*` constructor. Mocks are generated from those interfaces.
- Of the three layers, only `repository` imports GORM (`database` and `testutil/pgtest` also do, as infrastructure). Only `handler` (and `server`) import Connect. `usecase` imports neither.
- `repository.HealthRepository` → `usecase.HealthUsecase` (with their tests) is the minimal reference slice. Copy its shape for new features. Its raw `Ping` error is classified as `Unavailable` by the usecase.

### Domain errors

`internal/domainerr` holds the error kinds usecases return: `NotFound`, `AlreadyExists`, `InvalidArgument`, `FailedPrecondition`, `Unavailable`, `DeadlineExceeded`, `ResourceExhausted`, `Internal`.

- `domainerr.New(kind, format, args...)` creates one. The message is **shown to clients**, so never put secrets or DB details in it.
- `domainerr.Wrap(cause, kind, format, args...)` attaches a cause, used for logs and `errors.Is`. The cause is never sent to clients.
- `domainerr.Is(err, kind)` / `domainerr.KindOf(err)` classify errors.
- `handler.ConnectError` is the **only** place kinds become `connect.Code`s. `Internal` and any unclassified error become `CodeInternal` with the generic message `internal error`; the real error is logged via `slog`.

### Metadata database

- GORM on the pgx driver (`gorm.io/driver/postgres`), opened by `database.Open` (pings with a timeout; `TranslateError` on; bound query values never logged) and closed by `database.Close`.
- Models with a single `id uuid` PK embed `repository.Model`. Its `BeforeCreate` hook assigns a UUIDv7 when `ID` is empty. Composite-key tables declare their own fields.
- **Never call `AutoMigrate`.** The schema is owned only by goose migrations.

### Migrations

Goose SQL files live in `backend/internal/database/migrations/`, embedded via `embed.FS` (`migrations.FS`). Applied versions are tracked in `goose_db_version`.

To add a migration, create the next sequentially numbered file, e.g. `00002_create_environments.sql`:

```sql
-- +goose Up
CREATE TABLE environments (...);

-- +goose Down
DROP TABLE environments;
```

Always write a working `Down`. Never edit a migration that has been merged; add a new one instead. Two branches that both add `0000N_*.sql` will collide on the version number at merge time, so renumber yours when that happens. New migrations are picked up automatically by `migrate`, and by `pgtest`'s template database in tests.

### Proto / codegen workflow

Proto files live under `backend/proto/<package>/<version>/*.proto` (e.g. `proto/dataplatform/v1/client.proto`), one directory per versioned package. `buf generate` reads `backend/buf.gen.yaml` and emits generated Go + Connect code into `backend/gen/` (gitignored-equivalent generated output — don't hand-edit it; regenerate instead). The `go_package_prefix` override in `buf.gen.yaml` maps every proto package to `github.com/bayu-aditya/data-platform/server/gen/...`.

To add a new RPC service:

1. Add the `.proto` under `backend/proto/<package>/<version>/`.
2. Run `buf lint && buf generate` from `backend/`.
3. Register the generated Connect handler on the mux in `internal/server/server.go`, and add the service's fully-qualified name to the `grpchealth.NewStaticChecker` call so it can be health-checked individually.

## Testing

Each layer has one test style:

| Layer | Style | Example |
|---|---|---|
| repository | Real Postgres via testcontainers (`internal/testutil/pgtest`) | `internal/repository/health_test.go`, `model_test.go` |
| usecase | Repository mocks generated by mockery | `internal/usecase/health_test.go` |
| handler | Real server on `127.0.0.1:0` with **mocked usecases**, called over the wire | `internal/server/server_test.go` shows the boot/poll/client pattern (no handler with usecases exists yet) |

Pure helpers with no dependencies, such as `config.Load`, `domainerr` and `handler.ConnectError`, get plain table-driven unit tests.

### Repository tests (testcontainers)

Each test package that touches the DB needs, once:

```go
func TestMain(m *testing.M) { pgtest.Main(m) }
```

- `pgtest.New(t) *gorm.DB` returns a **fresh, fully migrated database** for this test. It is cloned from a template DB migrated once per package, and dropped when the test ends. Tests are isolated and can use `t.Parallel()`.
- `pgtest.DSN(t)` returns the same kind of database as a connection string. `pgtest.EmptyDSN(t)` returns an unmigrated one, for testing migrations.
- One Postgres container (`postgres:17-alpine`) is started lazily per test package and stopped after its tests. Docker is the only requirement. Under `-short` these tests skip.

### Usecase tests (mockery)

`.mockery.yml` mocks every exported interface in `internal/repository` and `internal/usecase` into a sibling `mocks` package. For example, `repository.HealthRepository` becomes `repository/mocks.MockHealthRepository`, built with `mocks.NewMockHealthRepository(t)`, which asserts expectations on cleanup. Use the typed `EXPECT()` API:

```go
repo := mocks.NewMockHealthRepository(t)
repo.EXPECT().Ping(mock.Anything).Return(nil).Once()
```

After adding or changing an interface, run `go tool mockery` and commit the regenerated mocks. If a new package exposes interfaces other layers depend on, add it under `packages:` in `.mockery.yml`.

### Handler tests

Don't mock the transport. Boot the real server on `127.0.0.1:0`, poll the port until it accepts connections, then call it with a real client (e.g. `grpchealth.NewClient`, or a generated Connect client over unencrypted HTTP/2 with `connect.WithGRPC()`). Inject `usecase/mocks` into the handlers so each test controls the usecase results, including domain errors, and asserts the Connect code the client sees.
