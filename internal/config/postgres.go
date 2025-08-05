package config

import (
	"fmt"
	"strings"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DbName   string
}

func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:     GetEnvWithDefault("POSTGRES_HOST", "localhost"),
		Port:     GetEnvWithDefault("POSTGRES_PORT", "5432"),
		User:     GetEnvWithDefault("POSTGRES_USER", "postgres"),
		Password: GetEnvWithDefault("POSTGRES_PASS", "postgres"),
		DbName:   GetEnvWithDefault("DB_NAME", "postgres"),
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
