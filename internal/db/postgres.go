package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/db/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Callers need to make sure the pool is closed properly
func Open(postgresConfig config.PostgresConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(postgresConfig.String())
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

func Migrate(connString string) error {
	driver, err := iofs.New(migrations.MigrationsFs, ".")
	if err != nil {
		return fmt.Errorf("migration fs: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", driver, connString)
	if err != nil {
		return fmt.Errorf("creating migration instance: %v", err)
	}
	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("no migrations")
			return nil
		}
		return fmt.Errorf("applying migrations: %w", err)
	} else {
		log.Println("migrations applied")
	}
	return nil
}
