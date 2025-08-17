package main

import (
	"log"

	"github.com/arashthr/pensive/integrations/telegram"
	"github.com/arashthr/pensive/internal/config"
	"github.com/arashthr/pensive/internal/db"
	"github.com/arashthr/pensive/internal/logging"
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
