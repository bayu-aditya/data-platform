package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
)

// translateError maps the expected storage outcomes of a query on entity (a
// singular noun, e.g. "environment") to domainerr errors. Any other error is
// returned unchanged.
//
// Every repository should return its errors through this function, so later
// tables get the same mapping for free, in particular
// gorm.ErrForeignKeyViolated (a RESTRICT foreign key blocking a delete) as
// FailedPrecondition.
func translateError(err error, entity string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domainerr.Wrap(err, domainerr.NotFound, "%s not found", entity)
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return domainerr.Wrap(err, domainerr.AlreadyExists, "%s already exists", entity)
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return domainerr.Wrap(err, domainerr.FailedPrecondition, "%s is referenced by other resources", entity)
	default:
		return err
	}
}
