package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestClientIPExtractor_DirectModeIgnoresForwardedHeaders(t *testing.T) {
	extractor := NewClientIPExtractor(&config.Config{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:8080"
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	req.Header.Set(cloudflareClientIPHeader, "198.51.100.11")

	assert.Equal(t, "192.0.2.10", extractor(req))
}

func TestClientIPExtractor_CloudflareModeUsesValidClientHeader(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.CloudflareSecret = "origin-secret"
	extractor := NewClientIPExtractor(cfg)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:8080"
	req.Header.Set(cloudflareSecretHeader, "origin-secret")
	req.Header.Set(cloudflareClientIPHeader, "198.51.100.10")

	assert.Equal(t, "198.51.100.10", extractor(req))
}

func TestClientIPExtractor_CloudflareModeRequiresOriginSecret(t *testing.T) {
	testCases := []struct {
		name          string
		requestSecret string
		wantClientIP  string
	}{
		{name: "missing secret", wantClientIP: "192.0.2.10"},
		{name: "wrong secret", requestSecret: "wrong-secret", wantClientIP: "192.0.2.10"},
		{name: "matching secret", requestSecret: "origin-secret", wantClientIP: "198.51.100.10"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.HTTP.CloudflareSecret = "origin-secret"
			extractor := NewClientIPExtractor(cfg)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			req.RemoteAddr = "192.0.2.10:8080"
			req.Header.Set(cloudflareClientIPHeader, "198.51.100.10")
			if tc.requestSecret != "" {
				req.Header.Set(cloudflareSecretHeader, tc.requestSecret)
			}

			assert.Equal(t, tc.wantClientIP, extractor(req))
		})
	}
}

func TestClientIPExtractor_CloudflareModeFallsBackForInvalidHeader(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.CloudflareSecret = "origin-secret"
	extractor := NewClientIPExtractor(cfg)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:8080"
	req.Header.Set(cloudflareSecretHeader, "origin-secret")
	req.Header.Set(cloudflareClientIPHeader, "not-an-ip")

	assert.Equal(t, "192.0.2.10", extractor(req))
}

func TestRequestLogger_UsesOnlyAuthenticatedCloudflareClientIP(t *testing.T) {
	testCases := []struct {
		name          string
		requestSecret string
		wantRemoteIP  string
	}{
		{name: "missing secret", wantRemoteIP: "192.0.2.10"},
		{name: "wrong secret", requestSecret: "wrong-secret", wantRemoteIP: "192.0.2.10"},
		{name: "matching secret", requestSecret: "origin-secret", wantRemoteIP: "198.51.100.10"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			cfg := &config.Config{}
			cfg.Env.Debug = true
			cfg.HTTP.CloudflareSecret = "origin-secret"
			e.IPExtractor = NewClientIPExtractor(cfg)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/resource", nil)
			req.RemoteAddr = "192.0.2.10:8080"
			req.Header.Set(cloudflareClientIPHeader, "198.51.100.10")
			if tc.requestSecret != "" {
				req.Header.Set(cloudflareSecretHeader, tc.requestSecret)
			}
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			requestLogger := NewRequestLoggerMiddleware(logger, cfg)
			handler := requestLogger.Log(func(c echo.Context) error {
				return c.NoContent(http.StatusBadRequest)
			})

			assert.NoError(t, handler(ctx))
			assert.Contains(t, logs.String(), `"remote_ip":"`+tc.wantRemoteIP+`"`)
			if tc.wantRemoteIP != "198.51.100.10" {
				assert.NotContains(t, logs.String(), `"remote_ip":"198.51.100.10"`)
			}
		})
	}
}
