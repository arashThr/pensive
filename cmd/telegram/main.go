package main

import (
	"log"
	"log/slog"

	"github.com/arashthr/go-course/integrations/telegram"
	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/db"
)

func main() {
	slog.Info("Starting Telegram bot")

	configs, err := config.LoadEnvConfig()

	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	telegramToken := configs.Telegram.Token
	apiEndpoint := configs.Domain

	pool, err := db.Open(configs.PSQL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	telegram.StartBot(telegramToken, apiEndpoint, pool)
}
