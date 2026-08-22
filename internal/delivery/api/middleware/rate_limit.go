package middleware

import (
	"fmt"

	"radar/config"

	"github.com/google/uuid"
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
	if err := settings.ValidateAt("http.rateLimit"); err != nil {
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

// NewSessionRateLimiter creates the in-process limiter shared by refresh and logout routes.
// It keys requests by the refresh-token user ID and falls back to the client IP.
func NewSessionRateLimiter(
	cfg *config.Config,
	identify func(echo.Context) (uuid.UUID, bool),
) (echo.MiddlewareFunc, error) {
	settings := config.DefaultSessionRateLimitConfig()
	if cfg != nil && cfg.HTTP.SessionRateLimit != nil {
		settings = *cfg.HTTP.SessionRateLimit
	}
	if err := settings.ValidateAt("http.sessionRateLimit"); err != nil {
		return nil, fmt.Errorf("invalid session rate limit configuration: %w", err)
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
			if identify != nil {
				if userID, ok := identify(c); ok {
					return userID.String(), nil
				}
			}

			return c.RealIP(), nil
		},
	}), nil
}

// NewAPIRateLimiter creates the in-process limiter shared by authenticated API routes.
// It keys requests by authenticated user ID and falls back to the client IP when no user is present.
func NewAPIRateLimiter(cfg *config.Config) (echo.MiddlewareFunc, error) {
	settings := config.DefaultAPIRateLimitConfig()
	if cfg != nil && cfg.HTTP.APIRateLimit != nil {
		settings = *cfg.HTTP.APIRateLimit
	}
	if err := settings.ValidateAt("http.apiRateLimit"); err != nil {
		return nil, fmt.Errorf("invalid API rate limit configuration: %w", err)
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
			if userID, ok := GetUserID(c); ok {
				return userID.String(), nil
			}

			return c.RealIP(), nil
		},
	}), nil
}
