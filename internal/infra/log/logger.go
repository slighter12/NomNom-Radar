package logs

import (
	"log/slog"
	"os"
	"strings"

	"radar/config"

	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// Params 定義 logger 所需的參數
type Params struct {
	fx.In

	Config *config.Config
}

// New 創建並初始化 slog.Logger
func New(params Params) (*slog.Logger, error) {
	// 從配置解析日誌級別
	level, err := parseLogLevel(params.Config.Env.Log.Level)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse log level")
	}

	// 使用 JSON 格式和指定的日誌級別初始化 slog logger
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, nil
}

// parseLogLevel 將字符串日誌級別轉換為 slog.Level
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
