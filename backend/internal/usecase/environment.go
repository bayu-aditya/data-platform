package usecase

import (
	"context"
	"regexp"

	"github.com/google/uuid"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
)

// environmentNamePattern is the slug an environment name must match. Upstream
// callers send the name, so it is validated strictly and never changes.
var environmentNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// EnvironmentUsecase manages environments.
type EnvironmentUsecase interface {
	// Create makes a new environment. It returns InvalidArgument if name is not
	// a valid slug and AlreadyExists if the name is taken.
	Create(ctx context.Context, name string) (repository.Environment, error)
	// List returns all environments, oldest first.
	List(ctx context.Context) ([]repository.Environment, error)
	// Delete removes an environment. It returns InvalidArgument if id is not a
	// UUID, NotFound if there is no such environment and FailedPrecondition if
	// it is still referenced.
	Delete(ctx context.Context, id string) error
}

type environmentUsecase struct {
	repo repository.EnvironmentRepository
}

// NewEnvironmentUsecase returns an EnvironmentUsecase over repo.
func NewEnvironmentUsecase(repo repository.EnvironmentRepository) EnvironmentUsecase {
	return &environmentUsecase{repo: repo}
}

func (u *environmentUsecase) Create(ctx context.Context, name string) (repository.Environment, error) {
	if !environmentNamePattern.MatchString(name) {
		return repository.Environment{}, domainerr.New(domainerr.InvalidArgument,
			"environment name must match %s", environmentNamePattern)
	}
	return u.repo.Create(ctx, name)
}

func (u *environmentUsecase) List(ctx context.Context) ([]repository.Environment, error) {
	return u.repo.List(ctx)
}

func (u *environmentUsecase) Delete(ctx context.Context, id string) error {
	envID, err := uuid.Parse(id)
	if err != nil {
		return domainerr.New(domainerr.InvalidArgument, "environment id must be a UUID")
	}
	return u.repo.Delete(ctx, envID)
}
