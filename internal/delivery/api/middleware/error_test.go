package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/internal/delivery"
	"radar/internal/delivery/api/response"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorMiddleware_HandleHTTPError_SkipsCommittedResponse(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	middleware := NewErrorMiddleware(slog.Default())

	err := response.BadRequest(c, "VALIDATION_ERROR", "name is required")
	require.NoError(t, err)

	bodyBefore := rec.Body.String()

	middleware.HandleHTTPError(delivery.ErrResponseHandled, c)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, bodyBefore, rec.Body.String())
}

func TestErrorMiddleware_HandleHTTPError_IgnoresHandledResponseSentinel(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	middleware := NewErrorMiddleware(slog.Default())

	middleware.HandleHTTPError(delivery.ErrResponseHandled, c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
}
