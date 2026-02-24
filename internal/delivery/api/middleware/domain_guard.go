package middleware

import (
	"crypto/subtle"
	"net"
	"strings"

	"radar/config"
	"radar/internal/delivery/api/response"
	domainerrors "radar/internal/domain/errors"

	"github.com/labstack/echo/v4"
)

// DomainGuardMiddleware restricts requests to a single configured host.
type DomainGuardMiddleware struct {
	allowedHost      string
	cloudflareSecret string
}

// NewDomainGuardMiddleware creates a host validation middleware.
func NewDomainGuardMiddleware(cfg *config.Config) *DomainGuardMiddleware {
	return &DomainGuardMiddleware{
		allowedHost:      normalizeHost(cfg.HTTP.AllowedHost),
		cloudflareSecret: strings.TrimSpace(cfg.HTTP.CloudflareSecret),
	}
}

// ValidateHost blocks requests that do not pass origin validation.
func (m *DomainGuardMiddleware) ValidateHost(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Keep probe endpoint available for Cloud Run startup checks.
		if c.Request().URL.Path == "/health" {
			return next(c)
		}

		if m.allowedHost != "" {
			requestHost := normalizeHost(c.Request().Host)
			if requestHost != m.allowedHost {
				return appErrorResponse(c, domainerrors.ErrForbiddenHost)
			}
		}

		if m.cloudflareSecret != "" {
			clientSecret := strings.TrimSpace(c.Request().Header.Get(cloudflareSecretHeader))
			if !secretsEqual(clientSecret, m.cloudflareSecret) {
				return appErrorResponse(c, domainerrors.ErrForbiddenOrigin)
			}
		}

		return next(c)
	}
}

// #nosec G101 -- This is a public HTTP header name, not a credential.
const cloudflareSecretHeader = "X-Cloudflare-Secret"

func appErrorResponse(c echo.Context, appErr domainerrors.AppError) error {
	return response.Error(c, appErr.HTTPCode(), appErr.ErrorCode(), appErr.Message(), nil)
}

func secretsEqual(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func normalizeHost(rawHost string) string {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return ""
	}

	host = strings.TrimSuffix(host, ".")

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.Count(host, ":") == 1 {
		parts := strings.SplitN(host, ":", 2)
		host = parts[0]
	}

	return strings.ToLower(host)
}
