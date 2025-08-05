package logging

import (
	"fmt"

	"github.com/arashthr/go-course/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

var logger, _ = zap.NewProduction()
var DefaultLogger = logger.Sugar()

func getLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func Init(cfg *config.AppConfig) {
	initLogger(cfg)
	initTelegram(cfg)
}

func initLogger(cfg *config.AppConfig) error {
	var zapCfg zap.Config
	var encoderCfg zapcore.EncoderConfig

	if cfg.Environment == "development" {
		// Development: Human-readable console output, debug level
		zapCfg = zap.NewDevelopmentConfig()
		encoderCfg = zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder // Colored logs for readability
		zapCfg.EncoderConfig = encoderCfg
	} else {
		// Production: JSON output, info level, file rotation
		zapCfg = zap.NewProductionConfig()
		encoderCfg = zap.NewProductionEncoderConfig()
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
		zapCfg.EncoderConfig = encoderCfg
	}

	// Set log level from config
	zapCfg.Level = zap.NewAtomicLevelAt(getLevel(cfg.Logging.LogLevel))

	// Add caller information (file and line number)
	zapCfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Add stack traces for errors in development
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	// Build logger
	logger, err := zapCfg.Build(opts...)

	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}

	// Use SugaredLogger for simpler API
	Logger = logger.Sugar()

	return nil
}

// Sync flushes the logger
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}
