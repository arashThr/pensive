package loggercontext

import (
	"context"
	"fmt"
	"os"
	"runtime"

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
		source := "unknown"
		_, file, line, ok := runtime.Caller(1)
		if ok {
			source = fmt.Sprintf("%s:%d", file, line)
		}
		fmt.Fprintln(os.Stderr, "logger was not found in the context: "+source)
		return logging.DefaultLogger
	}
	return logger
}
