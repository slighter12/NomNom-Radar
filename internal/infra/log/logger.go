package logs

import (
	"log/slog"
	"os"
	"strings"

	"radar/config"

	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// Params defines the parameters required for the logger
type Params struct {
	fx.In

	Config *config.Config
}

// New creates and initializes slog.Logger
func New(params Params) (*slog.Logger, error) {
	// Parse log level from config
	level, err := parseLogLevel(params.Config.Env.Log.Level)
	if err != nil {
		return nil, err
	}

	// Initialize slog logger with JSON format and specified log level
	var logger *slog.Logger
	if params.Config.Env.Log.Pretty {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}

	return logger, nil
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, errors.Errorf("unknown log level: %s", level)
	}
}
