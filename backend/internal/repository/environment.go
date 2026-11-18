package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Environment is a global, user-named target such as "staging".
type Environment struct {
	Model
	Name string
}

// EnvironmentRepository stores environments.
type EnvironmentRepository interface {
	// Create inserts an environment named name. It returns a domainerr
	// AlreadyExists error if the name is taken.
	Create(ctx context.Context, name string) (Environment, error)
	// List returns all environments, oldest first.
	List(ctx context.Context) ([]Environment, error)
	// Delete removes the environment with the given id. It returns NotFound if
	// there is none and FailedPrecondition if other rows still reference it.
	Delete(ctx context.Context, id uuid.UUID) error
}

type environmentRepository struct {
	db *gorm.DB
}

// NewEnvironmentRepository returns an EnvironmentRepository backed by db.
func NewEnvironmentRepository(db *gorm.DB) EnvironmentRepository {
	return &environmentRepository{db: db}
}

func (r *environmentRepository) Create(ctx context.Context, name string) (Environment, error) {
	env := Environment{Name: name}
	if err := r.db.WithContext(ctx).Create(&env).Error; err != nil {
		return Environment{}, translateError(err, "environment")
	}
	return env, nil
}

func (r *environmentRepository) List(ctx context.Context) ([]Environment, error) {
	var envs []Environment
	// UUIDv7 ids are time-sortable, so ordering by id is creation order.
	if err := r.db.WithContext(ctx).Order("id").Find(&envs).Error; err != nil {
		return nil, translateError(err, "environment")
	}
	return envs, nil
}

func (r *environmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&Environment{}, "id = ?", id)
	if res.Error != nil {
		return translateError(res.Error, "environment")
	}
	if res.RowsAffected == 0 {
		return translateError(gorm.ErrRecordNotFound, "environment")
	}
	return nil
}
