// Package migrations embeds the goose SQL migrations for the metadata
// database. Add a migration by dropping a new NNNNN_description.sql file next
// to this one; it is picked up at build time.
package migrations

import "embed"

// FS holds every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
