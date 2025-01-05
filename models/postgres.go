package models

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Callers need to make sure the pool is closed properly
func Open(postgresConfig PostgresConfig) (*pgxpool.Pool, error) {
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

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DbName   string
}

func (cfg PostgresConfig) String() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DbName)
}
