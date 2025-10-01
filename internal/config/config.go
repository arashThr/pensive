package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type StripeConfig struct {
	Key                 string
	PriceId             string
	StripeWebhookSecret string
}

type TelegramConfig struct {
	BotName string
	Token   string
}

type TurnstileConfig struct {
	SiteKey   string
	SecretKey string
}

type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
}

type TelegramLoggerConfig struct {
	Token  string
	ChatID string
}

type LoggerConfig struct {
	LogLevel string
	LogFile  string
	Telegram TelegramLoggerConfig
}

type AppConfig struct {
	Environment string
	Domain      string
	PSQL        PostgresConfig
	SMTP        SMTPConfig
	CSRF        struct {
		Key    string
		Secure bool
	}
	Server struct {
		Address string
	}
	Logging   LoggerConfig
	Stripe    StripeConfig
	Telegram  TelegramConfig
	Turnstile TurnstileConfig
	GitHub    GitHubOAuthConfig
	Google    GoogleOAuthConfig
}

func LoadEnvConfig(envFiles ...string) (*AppConfig, error) {
	var cfg AppConfig
	err := godotenv.Load(envFiles...)
	if err != nil {
		return nil, fmt.Errorf("loading .env file: %w", err)
	}

	cfg.Domain = GetEnvOrDie("DOMAIN")
	cfg.Environment = GetEnvOrDie("ENVIRONMENT")

	// DB
	cfg.PSQL = DefaultPostgresConfig()

	// SMTP
	port, err := strconv.Atoi(GetEnvOrDie("SMTP_PORT"))
	if err != nil {
		return nil, err
	}
	cfg.SMTP = SMTPConfig{
		Host:     GetEnvOrDie("SMTP_HOST"),
		Port:     port,
		Username: GetEnvOrDie("SMTP_USER"),
		Password: GetEnvOrDie("SMTP_PASS"),
	}

	// CSRF
	cfg.CSRF.Key = GetEnvOrDie("CSRF_TOKEN")
	cfg.CSRF.Secure = GetEnvOrDie("CSRF_SECURE") == "true"

	// Server
	cfg.Server.Address = GetEnvOrDie("SERVER_ADDRESS")

	cfg.Logging = LoggerConfig{
		LogLevel: GetEnvWithDefault("LOG_LEVEL", "info"),
		LogFile:  GetEnvWithDefault("LOG_FILE", "./data/logs.log"),
		Telegram: TelegramLoggerConfig{
			Token:  os.Getenv("TELEGRAM_LOGGING_TOKEN"),
			ChatID: os.Getenv("TELEGRAM_LOGGING_CHAT_ID"),
		},
	}

	// Stripe
	cfg.Stripe = StripeConfig{
		Key:                 GetEnvOrDie("STRIPE_KEY"),
		PriceId:             GetEnvOrDie("STRIPE_PRICE_ID"),
		StripeWebhookSecret: GetEnvOrDie("STRIPE_WEBHOOK_SECRET"),
	}

	cfg.Telegram = TelegramConfig{
		BotName: GetEnvOrDie("TELEGRAM_BOT_NAME"),
		Token:   GetEnvOrDie("TELEGRAM_BOT_TOKEN"),
	}

	cfg.Turnstile = TurnstileConfig{
		SiteKey:   GetEnvOrDie("TURNSTILE_SITE_KEY"),
		SecretKey: GetEnvOrDie("TURNSTILE_SECRET_KEY"),
	}

	cfg.GitHub = GitHubOAuthConfig{
		ClientID:     GetEnvOrDie("GITHUB_CLIENT_ID"),
		ClientSecret: GetEnvOrDie("GITHUB_CLIENT_SECRET"),
	}

	cfg.Google = GoogleOAuthConfig{
		ClientID:     GetEnvOrDie("GOOGLE_CLIENT_ID"),
		ClientSecret: GetEnvOrDie("GOOGLE_CLIENT_SECRET"),
	}

	return &cfg, nil
}

func GetEnvWithDefault(envName, defaultValue string) string {
	if value := os.Getenv(envName); value != "" {
		return value
	}
	return defaultValue
}

func GetEnvOrDie(envName string) string {
	value := os.Getenv(envName)
	if value == "" {
		panic("Environment variable " + envName + " is not set")
	}
	return value
}
