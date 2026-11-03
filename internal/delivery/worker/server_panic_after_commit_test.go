package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestWorkerServer_PanicAfterCommittedResponseLogsOneErrorRecord(t *testing.T) {
	for _, debug := range []bool{false, true} {
		t.Run(map[bool]string{false: "debug off", true: "debug on"}[debug], func(t *testing.T) {
			var slogOutput, echoOutput bytes.Buffer
			cfg := &config.Config{}
			cfg.Env.Debug = debug
			srv, err := NewServer(ServerParams{
				Lc:     fxtest.NewLifecycle(t),
				Cfg:    cfg,
				Logger: slog.New(slog.NewJSONHandler(&slogOutput, &slog.HandlerOptions{Level: slog.LevelDebug})),
			})
			require.NoError(t, err)
			e := srv.(*workerServer).server
			e.Logger.SetOutput(&echoOutput)
			e.GET("/committed-panic", func(c echo.Context) error {
				if err := c.String(http.StatusOK, "partial"); err != nil {
					return err
				}
				panic(map[string]string{"password": "after-commit-secret", "full_address": "Quillfeather Alley"})
			})

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/committed-panic", nil))

			require.Equal(t, http.StatusOK, rec.Code)
			output := slogOutput.String()
			require.Equal(t, 1, strings.Count(output, "\n"), "want exactly one log record, got:\n%s", output)
			var record map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &record))
			assert.Equal(t, "HTTP request", record["msg"])
			assert.Equal(t, "ERROR", record["level"])
			assert.InDelta(t, http.StatusOK, record["status"], 0)
			assert.Equal(t, "recovered panic (map[string]string)", record["source_error_message"])
			stackValue, ok := record["stack"].(string)
			require.True(t, ok)
			assert.Contains(t, stackValue, "PanicAfterCommittedResponseLogsOneErrorRecord")
			captured := output + echoOutput.String()
			for _, secret := range []string{"after-commit-secret", "Quillfeather"} {
				assert.NotContains(t, captured, secret)
			}
		})
	}
}

func TestWorkerServer_SuccessfulRequestIsNotLoggedWithDebugOff(t *testing.T) {
	var slogOutput bytes.Buffer
	cfg := &config.Config{}
	srv, err := NewServer(ServerParams{
		Lc:     fxtest.NewLifecycle(t),
		Cfg:    cfg,
		Logger: slog.New(slog.NewJSONHandler(&slogOutput, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	require.NoError(t, err)
	e := srv.(*workerServer).server
	e.GET("/ok", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ok", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, slogOutput.String())
}
