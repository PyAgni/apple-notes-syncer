// Package logging provides a factory for creating configured zap loggers.
package logging

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a zap.Logger configured with the given level, format, and
// optional file path. If filePath is empty, logs go to stderr only.
func NewLogger(level, format, filePath string) (*zap.Logger, error) {
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("parsing log level %q: %w", level, err)
	}

	var cfg zap.Config
	switch format {
	case "json":
		cfg = zap.NewProductionConfig()
	default:
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	if filePath != "" {
		cfg.OutputPaths = append(cfg.OutputPaths, filePath)
		cfg.ErrorOutputPaths = append(cfg.ErrorOutputPaths, filePath)
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return logger, nil
}
