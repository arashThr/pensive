package context

import (
	"context"
	"fmt"
	"os"

	"github.com/arashthr/go-course/internal/logging"
	"github.com/arashthr/go-course/internal/models"
	"go.uber.org/zap"
)

type key int

const (
	userKey key = iota
	loggerKey
)

func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
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

func Logger(ctx context.Context) *zap.SugaredLogger {
	value := ctx.Value(loggerKey)
	logger, ok := value.(*zap.SugaredLogger)
	if !ok {
		fmt.Fprintln(os.Stderr, "logger was not found in the context")
		return logging.DefaultLogger
	}
	return logger
}
