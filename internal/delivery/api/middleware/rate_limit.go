package middleware

import (
	"fmt"

	"radar/config"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// NewAuthRateLimiter creates the in-process limiter shared by authentication routes.
// It returns nil when the limiter is explicitly disabled.
func NewAuthRateLimiter(cfg *config.Config) (echo.MiddlewareFunc, error) {
	settings := config.DefaultRateLimitConfig()
	if cfg != nil && cfg.HTTP.RateLimit != nil {
		settings = *cfg.HTTP.RateLimit
	}
	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("invalid authentication rate limit configuration: %w", err)
	}
	if !settings.IsEnabled() {
		return nil, nil
	}

	store := echomiddleware.NewRateLimiterMemoryStoreWithConfig(echomiddleware.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(*settings.Rate),
		Burst:     *settings.Burst,
		ExpiresIn: *settings.ExpiresIn,
	})

	return echomiddleware.RateLimiterWithConfig(echomiddleware.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
	}), nil
}
