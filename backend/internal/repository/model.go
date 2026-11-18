// Package repository is the data-access layer over the metadata database.
//
// Each repository is an exported interface (what usecases depend on and
// what mockery mocks) plus an unexported GORM implementation returned by a
// New* constructor. Repositories translate storage errors into domainerr
// errors; nothing above this layer sees GORM or SQL errors.
package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Model is the base for tables with a single-column UUID primary key. Embed
// it in GORM models:
//
//	type environment struct {
//		repository.Model
//		Name string
//	}
type Model struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BeforeCreate assigns a time-sortable UUIDv7 when the ID is empty, so
// insertion order and primary-key order agree.
func (m *Model) BeforeCreate(*gorm.DB) error {
	if m.ID != uuid.Nil {
		return nil
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate uuidv7: %w", err)
	}
	m.ID = id
	return nil
}
