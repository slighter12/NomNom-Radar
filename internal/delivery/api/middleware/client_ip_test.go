package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"

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
	req.Header.Set(cloudflareClientIPHeader, "198.51.100.10")

	assert.Equal(t, "198.51.100.10", extractor(req))
}

func TestClientIPExtractor_CloudflareModeFallsBackForInvalidHeader(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.CloudflareSecret = "origin-secret"
	extractor := NewClientIPExtractor(cfg)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:8080"
	req.Header.Set(cloudflareClientIPHeader, "not-an-ip")

	assert.Equal(t, "192.0.2.10", extractor(req))
}
