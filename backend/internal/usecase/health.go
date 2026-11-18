// Package usecase holds the business rules. Usecases depend only on
// repository interfaces, return domainerr errors, and know nothing about
// GORM, SQL or Connect.
//
// Each usecase is an exported interface (what handlers depend on and what
// mockery mocks) plus an unexported implementation returned by a New*
// constructor.
package usecase

import (
	"context"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
)

// HealthUsecase reports whether the service's own dependencies are usable.
//
// It is the smallest complete example of the usecase pattern (interface +
// implementation over a repository interface + mockery-based test).
type HealthUsecase interface {
	// CheckMetadataDB returns a domainerr.Unavailable error if the metadata
	// database cannot be queried.
	CheckMetadataDB(ctx context.Context) error
}

type healthUsecase struct {
	repo repository.HealthRepository
}

// NewHealthUsecase returns a HealthUsecase over repo.
func NewHealthUsecase(repo repository.HealthRepository) HealthUsecase {
	return &healthUsecase{repo: repo}
}

func (u *healthUsecase) CheckMetadataDB(ctx context.Context) error {
	if err := u.repo.Ping(ctx); err != nil {
		return domainerr.Wrap(err, domainerr.Unavailable, "metadata database unavailable")
	}
	return nil
}
