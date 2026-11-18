package repository_test

import (
	"context"
	"testing"

	"github.com/bayu-aditya/data-platform/server/internal/database"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/testutil/pgtest"
)

func TestHealthPingReachableDatabase(t *testing.T) {
	t.Parallel()

	repo := repository.NewHealthRepository(pgtest.New(t))

	if err := repo.Ping(t.Context()); err != nil {
		t.Errorf("ping: %v", err)
	}
}

func TestHealthPingClosedDatabase(t *testing.T) {
	t.Parallel()

	db := pgtest.New(t)
	if err := database.Close(db); err != nil {
		t.Fatalf("close: %v", err)
	}
	repo := repository.NewHealthRepository(db)

	if err := repo.Ping(context.Background()); err == nil {
		t.Error("ping on closed database: want error, got nil")
	}
}
