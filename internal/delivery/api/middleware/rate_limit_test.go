package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"radar/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiter_SharesPerIPBucketAcrossAuthGroups(t *testing.T) {
	e := echo.New()
	e.IPExtractor = NewClientIPExtractor(&config.Config{})
	cfg := &config.Config{}
	cfg.HTTP.RateLimit = newRateLimitConfig(1, 2, time.Minute)
	limiter, err := NewAuthRateLimiter(cfg)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	authGroup := e.Group("/auth")
	authGroup.Use(limiter)
	authGroup.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	oauthGroup := e.Group("/oauth")
	oauthGroup.Use(limiter)
	oauthGroup.POST("/google", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for _, path := range []string{"/auth/login", "/oauth/google"} {
		req := newRateLimitRequest(path, "192.0.2.1:1000")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}

	req := newRateLimitRequest("/auth/login", "192.0.2.1:1000")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	otherIPRequest := newRateLimitRequest("/auth/login", "192.0.2.2:1000")
	otherIPResponse := httptest.NewRecorder()
	e.ServeHTTP(otherIPResponse, otherIPRequest)
	assert.Equal(t, http.StatusNoContent, otherIPResponse.Code)
}

func TestAuthRateLimiter_Disabled(t *testing.T) {
	enabled := false
	cfg := &config.Config{}
	cfg.HTTP.RateLimit = &config.RateLimitConfig{Enabled: &enabled}

	limiter, err := NewAuthRateLimiter(cfg)
	require.NoError(t, err)
	assert.Nil(t, limiter)
}

func TestAuthRateLimiter_RejectsInvalidConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.RateLimit = newRateLimitConfig(-1, 1, time.Minute)

	limiter, err := NewAuthRateLimiter(cfg)
	assert.Nil(t, limiter)
	assert.Error(t, err)
}

func newRateLimitRequest(target, remoteAddr string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, nil)
	req.RemoteAddr = remoteAddr

	return req
}

func newRateLimitConfig(rate float64, burst int, expiresIn time.Duration) *config.RateLimitConfig {
	return &config.RateLimitConfig{
		Rate:      &rate,
		Burst:     &burst,
		ExpiresIn: &expiresIn,
	}
}
