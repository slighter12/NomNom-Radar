package middleware

import (
	"net"
	"net/http"
	"strings"

	"radar/config"

	"github.com/labstack/echo/v4"
)

const cloudflareClientIPHeader = "CF-Connecting-IP"

// NewClientIPExtractor returns a client IP extractor appropriate for the configured edge.
// Cloudflare-origin traffic uses its authenticated client-IP header; direct traffic ignores
// forwarded headers and uses the network peer address.
func NewClientIPExtractor(cfg *config.Config) echo.IPExtractor {
	directExtractor := echo.ExtractIPDirect()
	if cfg == nil || strings.TrimSpace(cfg.HTTP.CloudflareSecret) == "" {
		return directExtractor
	}

	return func(req *http.Request) string {
		if clientIP := net.ParseIP(strings.TrimSpace(req.Header.Get(cloudflareClientIPHeader))); clientIP != nil {
			return clientIP.String()
		}

		return directExtractor(req)
	}
}
