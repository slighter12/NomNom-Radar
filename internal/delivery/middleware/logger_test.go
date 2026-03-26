package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/api/response"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerMiddleware_SkipsHandledResponseSentinel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	cfg := &config.Config{}
	cfg.Env.Debug = true

	middleware := NewLoggerMiddleware(logger, cfg)
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := middleware.Handle(func(c echo.Context) error {
		err := response.BadRequest(c, "VALIDATION_ERROR", "name is required")
		require.NoError(t, err)

		return delivery.ErrResponseHandled
	})

	err := handler(c)

	require.ErrorIs(t, err, delivery.ErrResponseHandled)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
	assert.Equal(t, "HTTP Request", logEntry["msg"])
	assert.Equal(t, float64(http.StatusBadRequest), logEntry["status"])
	_, hasError := logEntry["error"]
	assert.False(t, hasError)
}
