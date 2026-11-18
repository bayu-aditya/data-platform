package database

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/bayu-aditya/data-platform/server/internal/database/migrations"
)

// NewMigrator returns a goose provider over the embedded migrations. Applied
// versions are recorded in the goose_db_version table.
func NewMigrator(db *sql.DB) (*goose.Provider, error) {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return provider, nil
}
