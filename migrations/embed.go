// Package migrations embeds the SQL migration files so the server can apply
// them at startup. The files remain usable by the golang-migrate CLI as well.
package migrations

import "embed"

// FS holds every versioned SQL migration.
//
//go:embed *.sql
var FS embed.FS
