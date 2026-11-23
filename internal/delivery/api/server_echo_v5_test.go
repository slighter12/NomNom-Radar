package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/config"
	"radar/internal/delivery/api/router"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func newTestAPIServer(t *testing.T, bodyLimit string) (*apiServer, error) {
	t.Helper()

	cfg := &config.Config{}
	cfg.HTTP.MaxRequestBodySize = bodyLimit
	srv, err := NewServer(ServerParams{
		Lc:           fxtest.NewLifecycle(t),
		Cfg:          cfg,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		RouterParams: router.RouterParams{Config: cfg},
	})
	if err != nil {
		return nil, err
	}

	return srv.(*apiServer), nil
}

// Echo v5 sentinel routing errors are no longer *echo.HTTPError, so an
// unmatched route or method must still map to its own 4xx instead of a 500.
func TestServer_RoutingSentinelErrorsKeepTheirStatus(t *testing.T) {
	t.Parallel()

	srv, err := newTestAPIServer(t, "1M")
	require.NoError(t, err)
	srv.server.GET("/probe", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	testCases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "not_found", method: http.MethodGet, path: "/missing", wantStatus: http.StatusNotFound},
		{name: "method_not_allowed", method: http.MethodPost, path: "/probe", wantStatus: http.StatusMethodNotAllowed},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			srv.server.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

func TestServer_BodyLimitIsParsedFromConfiguredSize(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		limit   string
		allowed int
	}{
		{limit: "1KB", allowed: 1000}, // SI: matches Echo v4 / gommon
		{limit: "1KiB", allowed: 1024},
	} {
		t.Run(tc.limit, func(t *testing.T) {
			srv, err := newTestAPIServer(t, tc.limit)
			require.NoError(t, err)
			srv.server.POST("/echo-body", func(c *echo.Context) error {
				if _, err := io.ReadAll(c.Request().Body); err != nil {
					return err
				}

				return c.NoContent(http.StatusNoContent)
			})

			post := func(size int) int {
				req := httptest.NewRequestWithContext(
					context.Background(), http.MethodPost, "/echo-body", strings.NewReader(strings.Repeat("a", size)),
				)
				rec := httptest.NewRecorder()
				srv.server.ServeHTTP(rec, req)

				return rec.Code
			}

			assert.Equal(t, http.StatusNoContent, post(tc.allowed))
			assert.Equal(t, http.StatusRequestEntityTooLarge, post(tc.allowed+1))
		})
	}
}

// Expected values were produced by labstack/gommon/bytes.Parse, the parser
// Echo v4's BodyLimit used, so existing configuration keeps its meaning.
func TestParseBodyLimit(t *testing.T) {
	t.Parallel()

	valid := map[string]int64{
		"512": 512, "1B": 1, "1K": 1000, "1KB": 1000, "1kb": 1000, "1 KB": 1000, "100KB": 100_000,
		"1M": 1_000_000, "1MB": 1_000_000, "1.5K": 1500, "2G": 2_000_000_000, "1T": 1_000_000_000_000,
		"1KiB": 1024, "1Ki": 1024, "1MiB": 1 << 20, "2 GiB": 2 << 30, "1.5KiB": 1536,
	}
	for raw, want := range valid {
		got, err := parseBodyLimit(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}

	for _, raw := range []string{"", "KB", "10XB", "-1K", "1..5M", " 1K", "1K ", "99999E"} {
		_, err := parseBodyLimit(raw)
		assert.Error(t, err, raw)
	}
}
