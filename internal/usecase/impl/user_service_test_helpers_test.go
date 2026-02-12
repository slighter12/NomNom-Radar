package impl

import (
	"io"
	"log/slog"

	"radar/config"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestConfig(maxActiveSessions int) *config.Config {
	return &config.Config{
		Auth: &config.AuthConfig{
			BcryptCost:        12,
			MaxActiveSessions: maxActiveSessions,
		},
	}
}
