package observability

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(level string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// Parse log level
	parsedLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		parsedLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(parsedLevel)

	// JSON encoder for structured logs
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func NewDevelopmentLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, _ := config.Build()
	return logger
}

func GetLoggerFromEnv() *zap.Logger {
	if os.Getenv("GO_ENV") == "development" {
		return NewDevelopmentLogger()
	}

	logger, err := NewLogger("info")
	if err != nil {
		// Fallback to development logger
		return NewDevelopmentLogger()
	}

	return logger
}
