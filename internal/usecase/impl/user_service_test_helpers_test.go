package impl

import (
	"log/slog"

	"radar/config"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newTestConfig(maxActiveSessions int) *config.Config {
	return &config.Config{
		Auth: &config.AuthConfig{
			Argon2Memory:        65536,
			Argon2Iterations:    3,
			Argon2Parallelism:   2,
			Argon2MaxConcurrent: 4,
			MaxActiveSessions:   maxActiveSessions,
		},
		LoginThrottle: &config.LoginThrottleConfig{
			MaxAttempts:      5,
			LockoutDecayDays: 7,
		},
	}
}
