package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/db/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
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

// TODO: Move it to configs or utils
func getEnvWithDefault(envName, defaultValue string) string {
	if value := os.Getenv(envName); value != "" {
		return value
	}
	return defaultValue
}

func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:     getEnvWithDefault("POSTGRES_HOST", "localhost"),
		Port:     getEnvWithDefault("POSTGRES_PORT", "5432"),
		User:     getEnvWithDefault("POSTGRES_USER", "postgres"),
		Password: getEnvWithDefault("POSTGRES_PASS", "postgres"),
		DbName:   getEnvWithDefault("DB_NAME", "postgres"),
	}
}

func (cfg PostgresConfig) PgConnectionString(options ...string) string {
	options = append(options, "sslmode=disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DbName, strings.Join(options, "&"))
}

func (cfg PostgresConfig) String() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DbName)
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
