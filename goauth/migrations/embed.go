// Package migrations embeds all SQL migration files so they can be run at
// startup without requiring the files to be present on disk.
package migrations

import "embed"

// FS holds all *.sql migration files.
//
//go:embed *.sql
var FS embed.FS
