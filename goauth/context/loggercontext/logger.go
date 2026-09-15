package loggercontext

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
)

type key string

const loggerKey key = "loggerKey"

// fallback is a no-op logger used when none is found in the context.
var fallback, _ = zap.NewProduction()

// WithLogger stores a logger in the context.
func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Logger retrieves the logger from the context.
// Falls back to a production zap logger and prints a warning if none is set.
func Logger(ctx context.Context) *zap.SugaredLogger {
	value := ctx.Value(loggerKey)
	logger, ok := value.(*zap.SugaredLogger)
	if !ok {
		fmt.Fprintln(os.Stderr, "goauth: logger not found in context, using fallback")
		return fallback.Sugar()
	}
	return logger
}
