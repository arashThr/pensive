package logging

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/arashthr/pensive/internal/config"
	"go.uber.org/zap/zapcore"
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

// telegramCore is a zapcore.Core that forwards error-level log entries to Telegram.
type telegramCore struct {
	tg *telegramLogging
}

func (c *telegramCore) Enabled(lvl zapcore.Level) bool {
	return c.tg.enabled && lvl >= zapcore.ErrorLevel
}

func (c *telegramCore) With(_ []zapcore.Field) zapcore.Core { return c }

func (c *telegramCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *telegramCore) Write(entry zapcore.Entry, _ []zapcore.Field) error {
	msg := fmt.Sprintf("[%s] %s", strings.ToUpper(entry.Level.String()), entry.Message)
	go c.tg.SendMessage(msg) // fire-and-forget; never block the log call
	return nil
}

func (c *telegramCore) Sync() error { return nil }

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
