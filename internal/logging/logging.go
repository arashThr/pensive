package logging

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/arashthr/go-course/internal/auth/context"
)

type loggerKey string

const LoggerKey = loggerKey("loggerKey")

var (
	instance *slog.Logger
	once     sync.Once
)

func GetLogger(isProduction bool) *slog.Logger {
	once.Do(func() {
		var handler slog.Handler
		if isProduction {
			handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level:     slog.LevelInfo,
				AddSource: true,
			})
		} else {
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
		}
		instance = slog.New(handler)
	})
	return instance
}

func LoggerMiddleware(isProduction bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			baseLogger := GetLogger(isProduction)
			ctx := r.Context()
			user := context.User(ctx)
			if user == nil {
				ctx = context.WithLogger(ctx, baseLogger)
			} else {
				reqLogger := baseLogger.With("user", user.ID)
				ctx = context.WithLogger(ctx, reqLogger)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
