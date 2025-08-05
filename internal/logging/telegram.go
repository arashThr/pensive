package logging

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/arashthr/go-course/internal/config"
)

type telegramLogging struct {
	enabled  bool
	endpoint string
	chatId   string
}

var Telegram telegramLogging

func initTelegram(configs *config.AppConfig) {
	enabled := false
	if configs.Logging.Telegram.Token != "" && configs.Logging.Telegram.ChatID != "" {
		enabled = true
	}
	endpoint := "https://api.telegram.org/bot" + configs.Logging.Telegram.Token + "/sendMessage"
	Telegram = telegramLogging{
		endpoint: endpoint,
		chatId:   configs.Logging.Telegram.ChatID,
		enabled:  enabled,
	}
}

func (tg *telegramLogging) SendMessage(message string) error {
	if !tg.enabled {
		return fmt.Errorf("telegram logging is disabled")
	}
	body := fmt.Appendf(nil, `{
		"text": "%s",
		"chat_id": "%s"
	}`, message, tg.chatId)
	req, err := http.Post(tg.endpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("telegram message: %w", err)
	}
	if req.StatusCode != 200 {
		return fmt.Errorf("telegram message status failed with %s", req.Status)
	}
	return nil
}
