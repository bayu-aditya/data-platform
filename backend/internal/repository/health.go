package repository

import (
	"context"

	"gorm.io/gorm"
)

// HealthRepository probes the metadata database.
//
// It is the smallest complete example of the repository pattern (interface
// + GORM implementation + testcontainers test) and the dependency of the
// sample usecase.HealthUsecase.
type HealthRepository interface {
	// Ping returns nil if the metadata database answers a query.
	Ping(ctx context.Context) error
}

type healthRepository struct {
	db *gorm.DB
}

// NewHealthRepository returns a HealthRepository backed by db.
func NewHealthRepository(db *gorm.DB) HealthRepository {
	return &healthRepository{db: db}
}

func (r *healthRepository) Ping(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec("SELECT 1").Error
}
