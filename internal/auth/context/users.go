package context

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/arashthr/go-course/internal/models"
)

type key int

const (
	userKey key = iota
	loggerKey
)

func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func User(ctx context.Context) *models.User {
	val := ctx.Value(userKey)
	user, ok := val.(*models.User)
	if !ok {
		// Most likely user context was not set
		return nil
	}
	return user
}

func Logger(ctx context.Context) *slog.Logger {
	value := ctx.Value(loggerKey)
	logger, ok := value.(*slog.Logger)
	if !ok {
		fmt.Fprintln(os.Stderr, "logger was not found in the context")
		return nil
	}
	return logger
}
