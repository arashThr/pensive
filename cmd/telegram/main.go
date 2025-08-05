package main

import (
	"log"

	"github.com/arashthr/go-course/integrations/telegram"
	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/db"
	"github.com/arashthr/go-course/internal/logging"
)

func main() {
	configs, err := config.LoadEnvConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logging.Init(configs)
	logging.Logger.Infow("Starting Telegram bot")

	telegramToken := configs.Telegram.Token
	apiEndpoint := configs.Domain
	logging.Logger.Infow("API endpoint", "endpoint", apiEndpoint)

	pool, err := db.Open(configs.PSQL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	telegram.StartBot(telegramToken, apiEndpoint, pool)
}
