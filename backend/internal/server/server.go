// Package server wires the Connect/gRPC HTTP handlers into an http.Server.
package server

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/grpchealth"
)

// Config holds the runtime options for the RPC server.
type Config struct {
	// Addr is the TCP address to listen on, e.g. ":8080".
	Addr string
	// ReadHeaderTimeout bounds how long a client may take to send request
	// headers. Zero falls back to DefaultReadHeaderTimeout.
	ReadHeaderTimeout time.Duration
}

// DefaultReadHeaderTimeout is used when Config.ReadHeaderTimeout is zero.
const DefaultReadHeaderTimeout = 10 * time.Second

// Server is the RPC server together with the handlers registered on it.
type Server struct {
	http   *http.Server
	health *grpchealth.StaticChecker
}

// New builds a server that speaks gRPC, gRPC-Web and the Connect protocol over
// h2c (HTTP/2 without TLS), so a plain gRPC client can talk to it directly.
//
// The only registered service is grpc.health.v1.Health. Register additional
// generated Connect handlers on the mux as services are added, and list their
// fully-qualified names in the health checker so they can be probed
// individually.
func New(cfg Config) *Server {
	mux := http.NewServeMux()

	// No service names yet: an empty checker reports the process as SERVING
	// for a check without a service name, and NOT_FOUND for any named one.
	health := grpchealth.NewStaticChecker()
	mux.Handle(grpchealth.NewHandler(health))

	readHeaderTimeout := cfg.ReadHeaderTimeout
	if readHeaderTimeout == 0 {
		readHeaderTimeout = DefaultReadHeaderTimeout
	}

	// gRPC needs HTTP/2. Allowing it without TLS (h2c) keeps local and
	// in-cluster clients simple; terminate TLS in front of this in production.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	return &Server{
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           mux,
			Protocols:         protocols,
			ReadHeaderTimeout: readHeaderTimeout,
		},
		health: health,
	}
}

// Health exposes the checker so callers can flip service statuses, for example
// to report NOT_SERVING while draining.
func (s *Server) Health() *grpchealth.StaticChecker {
	return s.health
}

// Addr reports the configured listen address.
func (s *Server) Addr() string {
	return s.http.Addr
}

// ListenAndServe blocks until the server stops. It returns
// http.ErrServerClosed after a call to Shutdown.
func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server, waiting for in-flight requests until
// ctx is done.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
