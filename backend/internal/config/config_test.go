package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bayu-aditya/data-platform/server/internal/config"
)

func envOf(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envOf(map[string]string{
		"DATABASE_URL": "postgres://localhost/app",
	}))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	want := config.Config{
		GRPCAddr:     ":8080",
		DatabaseURL:  "postgres://localhost/app",
		QueryTimeout: 30 * time.Second,
		QueryMaxRows: 100000,
	}
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envOf(map[string]string{
		"GRPC_ADDR":      "127.0.0.1:9000",
		"DATABASE_URL":   "postgres://db/app",
		"QUERY_TIMEOUT":  "5s",
		"QUERY_MAX_ROWS": "42",
	}))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	want := config.Config{
		GRPCAddr:     "127.0.0.1:9000",
		DatabaseURL:  "postgres://db/app",
		QueryTimeout: 5 * time.Second,
		QueryMaxRows: 42,
	}
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		vars    map[string]string
		wantErr string
	}{
		{
			name:    "missing database url",
			vars:    map[string]string{},
			wantErr: "DATABASE_URL: is required",
		},
		{
			name:    "unparsable timeout",
			vars:    map[string]string{"DATABASE_URL": "x", "QUERY_TIMEOUT": "soon"},
			wantErr: `QUERY_TIMEOUT: must be a positive duration such as 30s, got "soon"`,
		},
		{
			name:    "negative timeout",
			vars:    map[string]string{"DATABASE_URL": "x", "QUERY_TIMEOUT": "-1s"},
			wantErr: "QUERY_TIMEOUT: must be a positive duration",
		},
		{
			name:    "non-numeric max rows",
			vars:    map[string]string{"DATABASE_URL": "x", "QUERY_MAX_ROWS": "lots"},
			wantErr: `QUERY_MAX_ROWS: must be a positive integer, got "lots"`,
		},
		{
			name:    "zero max rows",
			vars:    map[string]string{"DATABASE_URL": "x", "QUERY_MAX_ROWS": "0"},
			wantErr: "QUERY_MAX_ROWS: must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(envOf(tt.vars))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	t.Parallel()

	_, err := config.Load(envOf(map[string]string{
		"QUERY_TIMEOUT":  "soon",
		"QUERY_MAX_ROWS": "lots",
	}))
	if err == nil {
		t.Fatal("want error, got nil")
	}
	for _, name := range []string{"DATABASE_URL", "QUERY_TIMEOUT", "QUERY_MAX_ROWS"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("err = %q, want it to mention %s", err, name)
		}
	}
}
