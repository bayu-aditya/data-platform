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
go test ./...                    # internal/server tests boot a real server on an ephemeral port
go test ./... -run TestName       # run a single test
buf lint                          # lint proto files under proto/
buf generate                      # regenerate gen/ from proto/
```

`protoc-gen-go` and `protoc-gen-connect-go` are pinned via `tool` directives in `backend/go.mod`, so `buf generate` needs no global plugin installs.

Run the server:

```bash
go run ./cmd/server
# GRPC_ADDR env var overrides the default :8080 listen address
```

## Architecture

### Backend server

`backend/internal/server/server.go` builds an `http.Server` that speaks gRPC, gRPC-Web, and Connect simultaneously on one port over h2c (HTTP/2 without TLS) — a plain gRPC client can connect directly, no separate gateway. `server.New(cfg)` registers handlers on an `http.ServeMux` and returns a `*Server` with `ListenAndServe`/`Shutdown`. `backend/cmd/server/main.go` is the entrypoint: it reads `GRPC_ADDR`, starts the server, and shuts it down gracefully on SIGINT/SIGTERM within a bounded timeout.

Service health is reported via the standard `grpc.health.v1.Health` protocol (`connectrpc.com/grpchealth`), backed by a `grpchealth.StaticChecker` exposed through `Server.Health()`. A check with no service name always reports the process-wide status; a named check only succeeds for services explicitly passed to `grpchealth.NewStaticChecker`.

### Proto / codegen workflow

Proto files live under `backend/proto/<package>/<version>/*.proto` (e.g. `proto/dataplatform/v1/client.proto`), one directory per versioned package. `buf generate` reads `backend/buf.gen.yaml` and emits generated Go + Connect code into `backend/gen/` (gitignored-equivalent generated output — don't hand-edit it; regenerate instead). The `go_package_prefix` override in `buf.gen.yaml` maps every proto package to `github.com/bayu-aditya/data-platform/server/gen/...`.

To add a new RPC service:

1. Add the `.proto` under `backend/proto/<package>/<version>/`.
2. Run `buf lint && buf generate` from `backend/`.
3. Register the generated Connect handler on the mux in `internal/server/server.go`, and add the service's fully-qualified name to the `grpchealth.NewStaticChecker` call so it can be health-checked individually.

### Testing pattern

Server tests (`backend/internal/server/server_test.go`) don't use mocks: they start the real server on `127.0.0.1:0`, poll the port until it accepts connections, then exercise it with a real gRPC client (`grpchealth.NewClient` over unencrypted HTTP/2). Follow this pattern for new service tests — boot the real server, talk to it over the wire.
