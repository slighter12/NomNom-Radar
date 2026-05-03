package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/api/response"
	domainerrors "radar/internal/domain/errors"

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

func TestErrorMiddleware_HandleHTTPError_LogsSanitizedRequestAndResponseError(t *testing.T) {
	e := echo.New()
	body := strings.NewReader(`{
		"email":"owner@example.com",
		"password":"Password123!",
		"business_license":"A123456789",
		"store_name":"Secret Shop",
		"debug":"debug@example.com"
	}`)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/auth/register/merchant?email=owner@example.com&debug=true",
		body,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer secret-token")
	req.Header.Set("User-Agent", "test-agent")

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	requestLogger := NewRequestLoggerMiddleware(logger, &config.Config{})
	errorMiddleware := NewErrorMiddleware(logger)
	handler := requestLogger.Log(errorMiddleware.HandleErrors(CaptureRequestBodyForErrorLog(func(c echo.Context) error {
		return domainerrors.ErrInvalidCredentials
	})))

	err := handler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	logOutput := logs.String()
	assert.Equal(t, 1, strings.Count(logOutput, "\n"))
	assert.Contains(t, logOutput, "HTTP request")
	assert.Contains(t, logOutput, "INVALID_CREDENTIALS")
	assert.Contains(t, logOutput, "debug@example.com")
	assert.Contains(t, logOutput, "debug")
	assert.NotContains(t, logOutput, "owner@example.com")
	assert.NotContains(t, logOutput, "Password123")
	assert.NotContains(t, logOutput, "A123456789")
	assert.NotContains(t, logOutput, "Secret Shop")
	assert.NotContains(t, logOutput, "secret-token")
}

func TestErrorMiddleware_HandleHTTPError_RedactsTokenLikeRequestFields(t *testing.T) {
	e := echo.New()
	body := strings.NewReader(`{
		"onboarding_token":"onboarding-secret",
		"linking_token":"linking-secret",
		"fcm_token":"fcm-secret",
		"nested":{"device-token":"nested-secret"},
		"visible":"safe-value"
	}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/link", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	requestLogger := NewRequestLoggerMiddleware(logger, &config.Config{})
	errorMiddleware := NewErrorMiddleware(logger)
	handler := requestLogger.Log(errorMiddleware.HandleErrors(CaptureRequestBodyForErrorLog(func(c echo.Context) error {
		return domainerrors.ErrValidationFailed
	})))

	err := handler(c)
	require.NoError(t, err)

	logOutput := logs.String()
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, logOutput, "onboarding_token")
	assert.Contains(t, logOutput, "linking_token")
	assert.Contains(t, logOutput, "fcm_token")
	assert.Contains(t, logOutput, "safe-value")
	assert.NotContains(t, logOutput, "onboarding-secret")
	assert.NotContains(t, logOutput, "linking-secret")
	assert.NotContains(t, logOutput, "fcm-secret")
	assert.NotContains(t, logOutput, "nested-secret")
}

func TestErrorMiddleware_HandleHTTPError_LogsStackForInternalError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/broken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	requestLogger := NewRequestLoggerMiddleware(logger, &config.Config{})
	errorMiddleware := NewErrorMiddleware(logger)
	handler := requestLogger.Log(errorMiddleware.HandleErrors(func(c echo.Context) error {
		return errors.New("database failed for owner@example.com authorization=Bearer secret-token")
	}))

	err := handler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	logOutput := logs.String()
	assert.Equal(t, 1, strings.Count(logOutput, "\n"))
	assert.Contains(t, logOutput, "INTERNAL_ERROR")
	assert.Contains(t, logOutput, "source_error_type")
	assert.Contains(t, logOutput, "stack")
	assert.NotContains(t, logOutput, "owner@example.com")
	assert.NotContains(t, logOutput, "secret-token")
}

func TestRequestLoggerMiddleware_LogsDirectSanitizedErrorResponse(t *testing.T) {
	e := echo.New()
	body := strings.NewReader(`{"email":"owner@example.com","password":"Password123!","debug":"visible"}`)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/auth/login?access_token=query-secret",
		body,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer secret-token")

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	requestLogger := NewRequestLoggerMiddleware(logger, &config.Config{})
	errorMiddleware := NewErrorMiddleware(logger)
	handler := requestLogger.Log(errorMiddleware.HandleErrors(CaptureRequestBodyForErrorLog(func(c echo.Context) error {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authorization header is missing")
	})))

	err := handler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	logOutput := logs.String()
	assert.Equal(t, 1, strings.Count(logOutput, "\n"))
	assert.Contains(t, logOutput, "UNAUTHORIZED")
	assert.Contains(t, logOutput, "Authorization header is missing")
	assert.Contains(t, logOutput, "visible")
	assert.NotContains(t, logOutput, "owner@example.com")
	assert.NotContains(t, logOutput, "Password123")
	assert.NotContains(t, logOutput, "query-secret")
	assert.NotContains(t, logOutput, "secret-token")
}

func TestRequestLoggerMiddleware_LogsHandledResponseSentinelOnce(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/bad", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	requestLogger := NewRequestLoggerMiddleware(logger, &config.Config{})
	errorMiddleware := NewErrorMiddleware(logger)
	handler := requestLogger.Log(errorMiddleware.HandleErrors(func(c echo.Context) error {
		err := response.BadRequest(c, "VALIDATION_ERROR", "name is required")
		require.NoError(t, err)

		return delivery.ErrResponseHandled
	}))

	err := handler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 1, strings.Count(logs.String(), "\n"))
	assert.Contains(t, logs.String(), "VALIDATION_ERROR")
}

func TestRequestLoggerMiddleware_SkipsSuccessfulRequestOutsideDebug(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	requestLogger := NewRequestLoggerMiddleware(logger, &config.Config{})
	handler := requestLogger.Log(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, logs.String())
}

func TestRequestLoggerMiddleware_LogsSuccessfulRequestInDebug(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	cfg := &config.Config{}
	cfg.Env.Debug = true
	requestLogger := NewRequestLoggerMiddleware(logger, cfg)
	handler := requestLogger.Log(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)
	require.NoError(t, err)

	logOutput := logs.String()
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, strings.Count(logOutput, "\n"))
	assert.Contains(t, logOutput, "HTTP request")
	assert.Contains(t, logOutput, `"status":200`)
}
