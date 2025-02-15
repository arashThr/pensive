package logger

import (
	"log/slog"
	"net/http"
	"os"
	"sync"
)

type loggerKey string

const LoggerKey = loggerKey("loggerKey")

var (
	instance *slog.Logger
	once     sync.Once
)

func GetLogger() *slog.Logger {
	once.Do(func() {
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		})
		instance = slog.New(handler)
	})
	return instance
}
