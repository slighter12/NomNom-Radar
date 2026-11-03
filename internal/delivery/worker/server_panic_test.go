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

func panickingWorkerHandler(echo.Context) error {
	panic("handler exploded token=panic-secret")
}

func TestWorkerServer_HandlerPanicProducesOneLifecycleErrorLog(t *testing.T) {
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
			e.GET("/panic-probe", panickingWorkerHandler)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic-probe", nil)
			req.Header.Set("X-Request-Id", "panic-req-1")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusInternalServerError, rec.Code)
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
				Meta struct {
					RequestID string `json:"request_id"`
				} `json:"meta"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "INTERNAL_ERROR", body.Error.Code)
			assert.Equal(t, "panic-req-1", body.Meta.RequestID)
			assert.NotContains(t, rec.Body.String(), "exploded")

			assertSinglePanicLifecycleLog(t, slogOutput.String(), "panic-req-1", "panickingWorkerHandler")
			assert.Empty(t, echoOutput.String(), "echo logger must not emit a duplicate panic log")
		})
	}
}

func assertSinglePanicLifecycleLog(t *testing.T, output, requestID, panicFunc string) {
	t.Helper()

	require.Equal(t, 1, strings.Count(output, "\n"), "want exactly one log record, got:\n%s", output)
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &record))
	assert.Equal(t, "HTTP request", record["msg"])
	assert.Equal(t, "ERROR", record["level"])
	assert.InDelta(t, http.StatusInternalServerError, record["status"], 0)
	assert.Equal(t, requestID, record["request_id"])
	responseError, ok := record["response_error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INTERNAL_ERROR", responseError["code"])
	stackValue, ok := record["stack"].(string)
	require.True(t, ok)
	assert.Contains(t, stackValue, panicFunc)
	assert.Equal(t, "recovered panic (string)", record["source_error_message"])
	assert.NotContains(t, output, "handler exploded")
	assert.NotContains(t, output, "panic-secret")
}
