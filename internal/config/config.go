package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/arashthr/go-course/internal/db"
	"github.com/arashthr/go-course/internal/service"
	"github.com/arashthr/go-course/internal/validations"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v81"
)

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

type AppConfig struct {
	Environment string
	Domain      string
	PSQL        db.PostgresConfig
	SMTP        service.SMTPConfig
	CSRF        struct {
		Key    string
		Secure bool
	}
	Server struct {
		Address string
	}
	Stripe    StripeConfig
	Telegram  TelegramConfig
	Turnstile TurnstileConfig
}

func LoadEnvConfig() (*AppConfig, error) {
	var cfg AppConfig
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("loading .env file: %w", err)
	}

	cfg.Domain = os.Getenv("DOMAIN")

	// DB
	cfg.PSQL = db.DefaultPostgresConfig()

	// SMTP
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		return nil, err
	}
	cfg.SMTP = service.SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     port,
		Username: os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASS"),
	}

	// CSRF
	cfg.CSRF.Key = os.Getenv("CSRF_TOKEN")
	cfg.CSRF.Secure = os.Getenv("CSRF_SECURE") == "true"

	// Server
	cfg.Server.Address = os.Getenv("SERVER_ADDRESS")

	// Stripe
	cfg.Stripe = StripeConfig{
		Key:                 os.Getenv("STRIPE_KEY"),
		PriceId:             os.Getenv("STRIPE_PRICE_ID"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
	// Or set stripe.Key
	stripe.Key = os.Getenv("STRIPE_KEY")

	cfg.Telegram = TelegramConfig{
		BotName: validations.GetEnvOrDie("TELEGRAM_BOT_NAME"),
		Token:   validations.GetEnvOrDie("TELEGRAM_BOT_TOKEN"),
	}

	cfg.Turnstile = TurnstileConfig{
		SiteKey:   validations.GetEnvOrDie("TURNSTILE_SITE_KEY"),
		SecretKey: validations.GetEnvOrDie("TURNSTILE_SECRET_KEY"),
	}

	return &cfg, nil
}
