# server

Connect RPC server for the data platform. It speaks the gRPC, gRPC-Web and
Connect protocols on a single port over unencrypted HTTP/2 (h2c), so a stock
gRPC client can connect without any gateway in between.

Today the only registered service is the standard
[gRPC health checking protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md)
(`grpc.health.v1.Health`), implemented by
[connectrpc.com/grpchealth](https://pkg.go.dev/connectrpc.com/grpchealth).

## Run

```bash
go run ./cmd/server
```

| Env var     | Default | Description               |
| ----------- | ------- | ------------------------- |
| `GRPC_ADDR` | `:8080` | TCP address to listen on. |

## Probe the health check

With [grpcurl](https://github.com/fullstorydev/grpcurl):

```bash
# Process-wide status.
grpcurl -plaintext localhost:8080 grpc.health.v1.Health/Check

# Per-service status; unregistered names return NOT_FOUND.
grpcurl -plaintext -d '{"service":"dataplatform.v1.SomeService"}' \
  localhost:8080 grpc.health.v1.Health/Check
```

A check without a service name reports `SERVING` for the process. Named checks
only succeed for services passed to `grpchealth.NewStaticChecker` in
[server.go](internal/server/server.go).

## Adding a service

1. Put the `.proto` file under `proto/`, one directory per package, e.g.
   `proto/dataplatform/v1/ingest.proto`.
2. Generate the Go and Connect code into `gen/`:

   ```bash
   buf lint
   buf generate
   ```

   The `protoc-gen-go` and `protoc-gen-connect-go` plugins are pinned by the
   `tool` directives in `go.mod`, so no global installs are required.
3. Register the generated handler on the mux in
   [server.go](internal/server/server.go), and add the service's
   fully-qualified name to `grpchealth.NewStaticChecker` so it can be probed
   individually.

## Test

```bash
go test ./...
```

The tests in `internal/server` boot the server on an ephemeral port and call
`Check` over the real gRPC protocol.
