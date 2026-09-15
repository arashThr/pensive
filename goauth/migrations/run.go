package migrations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run executes all *.up.sql migration files in lexicographic order.
// It is idempotent: each migration is wrapped in a transaction and
// the table goauth_migrations tracks which files have already run.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	// Ensure tracking table exists.
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS goauth_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create goauth_migrations table: %w", err)
	}

	entries, err := FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		var alreadyApplied bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM goauth_migrations WHERE filename = $1)`, name,
		).Scan(&alreadyApplied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if alreadyApplied {
			continue
		}

		sql, err := FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err = tx.Exec(ctx,
			`INSERT INTO goauth_migrations (filename) VALUES ($1)`, name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}
