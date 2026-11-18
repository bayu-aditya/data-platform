// Package config loads the server's runtime configuration from environment
// variables. Every env var the process reads is declared here, in one place.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Config is the fully parsed process configuration.
type Config struct {
	// GRPCAddr is the TCP address the RPC server listens on (GRPC_ADDR).
	GRPCAddr string
	// DatabaseURL is the metadata Postgres DSN (DATABASE_URL). Required.
	DatabaseURL string
	// QueryTimeout bounds a single data-path query (QUERY_TIMEOUT).
	QueryTimeout time.Duration
	// QueryMaxRows caps the rows a single data-path query may return
	// (QUERY_MAX_ROWS).
	QueryMaxRows int
}

// Load reads the configuration through getenv (os.Getenv in production) and
// reports every missing or malformed variable at once, so a misconfigured
// deployment fails fast with one clear error.
//
// To add a variable: add a field above and one line below using the matching
// env helper (or add a helper for a new type).
func Load(getenv func(string) string) (Config, error) {
	e := env{getenv: getenv}
	cfg := Config{
		GRPCAddr:     e.string("GRPC_ADDR", ":8080"),
		DatabaseURL:  e.required("DATABASE_URL"),
		QueryTimeout: e.positiveDuration("QUERY_TIMEOUT", 30*time.Second),
		QueryMaxRows: e.positiveInt("QUERY_MAX_ROWS", 100000),
	}
	if err := errors.Join(e.errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// env parses typed values out of environment variables, collecting errors
// instead of stopping at the first one. An empty variable counts as unset.
type env struct {
	getenv func(string) string
	errs   []error
}

func (e *env) fail(name, format string, args ...any) {
	e.errs = append(e.errs, fmt.Errorf("%s: %s", name, fmt.Sprintf(format, args...)))
}

func (e *env) string(name, def string) string {
	if v := e.getenv(name); v != "" {
		return v
	}
	return def
}

func (e *env) required(name string) string {
	v := e.getenv(name)
	if v == "" {
		e.fail(name, "is required")
	}
	return v
}

func (e *env) positiveDuration(name string, def time.Duration) time.Duration {
	raw := e.getenv(name)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		e.fail(name, "must be a positive duration such as 30s, got %q", raw)
		return def
	}
	return d
}

func (e *env) positiveInt(name string, def int) int {
	raw := e.getenv(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		e.fail(name, "must be a positive integer, got %q", raw)
		return def
	}
	return n
}
