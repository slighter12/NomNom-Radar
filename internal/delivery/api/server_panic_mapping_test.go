package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/config"
	"radar/internal/delivery/api/router"
	domainerrors "radar/internal/domain/errors"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestServer_RecoveredPanicAlwaysMapsToInternalError(t *testing.T) {
	for _, tc := range []struct {
		name       string
		value      any
		wantSource string
	}{
		{name: "string", value: "string panic value", wantSource: "string panic value"},
		{name: "plain error", value: errors.New("plain panic error"), wantSource: "plain panic error"},
		{name: "app error", value: domainerrors.ErrValidationFailed, wantSource: domainerrors.ErrValidationFailed.Error()},
		{name: "echo http error", value: echo.ErrForbidden, wantSource: echo.ErrForbidden.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			cfg := &config.Config{}
			cfg.HTTP.MaxRequestBodySize = "1M"
			srv, err := NewServer(ServerParams{
				Lc:           fxtest.NewLifecycle(t),
				Cfg:          cfg,
				Logger:       slog.New(slog.NewJSONHandler(&logs, nil)),
				RouterParams: router.RouterParams{Config: cfg},
			})
			require.NoError(t, err)
			e := srv.(*apiServer).server
			e.GET("/panic-mapping", func(echo.Context) error { panic(tc.value) })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic-mapping", nil))

			assertRecoveredPanicMappedToInternalError(t, rec, logs.String(), tc.wantSource)
		})
	}
}

func assertRecoveredPanicMappedToInternalError(t *testing.T, rec *httptest.ResponseRecorder, output, wantSource string) {
	t.Helper()

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"INTERNAL_ERROR"`)

	require.Equal(t, 1, strings.Count(output, "\n"), "want exactly one log record, got:\n%s", output)
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &record))
	assert.Equal(t, "ERROR", record["level"])
	assert.InDelta(t, http.StatusInternalServerError, record["status"], 0)
	responseError, ok := record["response_error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INTERNAL_ERROR", responseError["code"])
	stackValue, ok := record["stack"].(string)
	require.True(t, ok)
	assert.Contains(t, stackValue, "RecoveredPanicAlwaysMapsToInternalError")
	assert.Contains(t, record["source_error_message"], wantSource)
}
