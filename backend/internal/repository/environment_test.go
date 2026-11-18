package repository_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/testutil/pgtest"
)

func TestEnvironmentCreateAndList(t *testing.T) {
	t.Parallel()

	repo := repository.NewEnvironmentRepository(pgtest.New(t))

	staging, err := repo.Create(t.Context(), "staging")
	if err != nil {
		t.Fatalf("create staging: %v", err)
	}
	if staging.ID == uuid.Nil || staging.CreatedAt.IsZero() {
		t.Errorf("create did not fill id/timestamps: %+v", staging)
	}
	if _, err := repo.Create(t.Context(), "production"); err != nil {
		t.Fatalf("create production: %v", err)
	}

	got, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].Name != "staging" || got[1].Name != "production" {
		t.Errorf("list = %+v, want staging then production", got)
	}
}

func TestEnvironmentListEmpty(t *testing.T) {
	t.Parallel()

	got, err := repository.NewEnvironmentRepository(pgtest.New(t)).List(t.Context())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("list = %+v, want empty", got)
	}
}

func TestEnvironmentCreateDuplicateIsAlreadyExists(t *testing.T) {
	t.Parallel()

	repo := repository.NewEnvironmentRepository(pgtest.New(t))
	if _, err := repo.Create(t.Context(), "staging"); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := repo.Create(t.Context(), "staging")

	if !domainerr.Is(err, domainerr.AlreadyExists) {
		t.Errorf("err = %v, want kind %v", err, domainerr.AlreadyExists)
	}
}

func TestEnvironmentDelete(t *testing.T) {
	t.Parallel()

	repo := repository.NewEnvironmentRepository(pgtest.New(t))
	env, err := repo.Create(t.Context(), "staging")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Delete(t.Context(), env.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("list after delete = %+v, want empty", got)
	}
}

func TestEnvironmentDeleteUnknownIsNotFound(t *testing.T) {
	t.Parallel()

	repo := repository.NewEnvironmentRepository(pgtest.New(t))

	err := repo.Delete(t.Context(), uuid.New())

	if !domainerr.Is(err, domainerr.NotFound) {
		t.Errorf("err = %v, want kind %v", err, domainerr.NotFound)
	}
}

// Later tables reference environments with ON DELETE RESTRICT. This uses a
// throwaway referencing table to prove that the resulting foreign-key
// violation surfaces as FailedPrecondition.
func TestEnvironmentDeleteReferencedIsFailedPrecondition(t *testing.T) {
	t.Parallel()

	db := pgtest.New(t)
	if err := db.Exec(`CREATE TABLE env_refs (
		id             uuid PRIMARY KEY,
		environment_id uuid NOT NULL REFERENCES environments (id) ON DELETE RESTRICT
	)`).Error; err != nil {
		t.Fatalf("create referencing table: %v", err)
	}
	repo := repository.NewEnvironmentRepository(db)
	env, err := repo.Create(t.Context(), "staging")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.Exec("INSERT INTO env_refs (id, environment_id) VALUES (?, ?)", uuid.New(), env.ID).Error; err != nil {
		t.Fatalf("insert reference: %v", err)
	}

	err = repo.Delete(t.Context(), env.ID)

	if !domainerr.Is(err, domainerr.FailedPrecondition) {
		t.Errorf("err = %v, want kind %v", err, domainerr.FailedPrecondition)
	}
}
