package loggercontext

import (
	"context"
	"fmt"
	"os"

	"github.com/arashthr/pensive/internal/logging"
	"go.uber.org/zap"
)

type key string

const loggerKey key = "loggerKey"

func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
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
