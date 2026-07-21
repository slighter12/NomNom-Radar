package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"radar/config"
	"radar/internal/delivery/api/response"
	"radar/internal/platform/observability"

	"github.com/labstack/echo/v4"
	"github.com/slighter12/go-lib/errors/stack"
)

const sourceErrorLogContextKey = "source_error_for_log"

type sourceErrorLog struct {
	Type    string
	Message string
	Stack   stack.Provider
}

// RequestLoggerMiddleware logs one request lifecycle entry after the response is finalized.
type RequestLoggerMiddleware struct {
	logger *slog.Logger
	debug  bool
}

func NewRequestLoggerMiddleware(logger *slog.Logger, cfg *config.Config) *RequestLoggerMiddleware {
	return &RequestLoggerMiddleware{
		logger: logger,
		debug:  cfg.Env.Debug,
	}
}

func (m *RequestLoggerMiddleware) Log(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		start := time.Now()
		defer func() {
			m.logRequest(c, start)
		}()

		return next(c)
	}
}

func (m *RequestLoggerMiddleware) logRequest(c echo.Context, start time.Time) {
	status := c.Response().Status
	if status == 0 {
		status = http.StatusOK
	}
	if status < http.StatusBadRequest && !m.debug {
		return
	}

	req := c.Request()
	attrs := []slog.Attr{
		slog.String("request_id", requestIDForLog(c)),
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.Int("status", status),
		slog.Duration("latency", time.Since(start)),
		slog.String("remote_ip", c.RealIP()),
		slog.String("user_agent", req.UserAgent()),
	}

	level := slog.LevelInfo
	if status >= http.StatusBadRequest {
		level = slog.LevelWarn
		attrs = append(attrs,
			slog.Any("request", buildSanitizedRequestLog(c)),
			slog.Any("response_error", responseErrorLogFromContext(c, status)),
		)
		if sourceErr, ok := c.Get(sourceErrorLogContextKey).(sourceErrorLog); ok {
			attrs = append(attrs, slog.String("source_error_type", sourceErr.Type))
			if sourceErr.Message != "" {
				attrs = append(attrs, slog.String("source_error_message", sourceErr.Message))
			}
		}
	}
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
		attrs = append(attrs, slog.String("stack", stackForInternalServerError(c)))
	}

	m.logger.LogAttrs(c.Request().Context(), level, "HTTP request", attrs...)
}

func requestIDForLog(c echo.Context) string {
	if id := observability.CorrelationIDFromContext(c.Request().Context()); id != "" {
		return id
	}

	return ""
}

func setSourceErrorLog(c echo.Context, err error) {
	if err == nil {
		return
	}

	logErr := err
	sourceErr := sourceErrorLog{}

	if stackProvider, ok := errors.AsType[stack.Provider](err); ok {
		sourceErr.Stack = stackProvider
		if wrappedErr := errors.Unwrap(stackProvider); wrappedErr != nil {
			logErr = wrappedErr
		}
	}
	sourceErr.Type = fmt.Sprintf("%T", logErr)
	sourceErr.Message = sanitizeFreeTextLogValue(logErr.Error())

	c.Set(sourceErrorLogContextKey, sourceErr)
}

func stackForInternalServerError(c echo.Context) string {
	if sourceErr, ok := c.Get(sourceErrorLogContextKey).(sourceErrorLog); ok && sourceErr.Stack != nil {
		if stack := sourceErr.Stack.Stack(); stack != "" {
			return stack
		}
	}

	return stack.Capture(0)
}

func responseErrorLogFromContext(c echo.Context, status int) responseErrorLog {
	if payload, ok := c.Get(response.ErrorLogContextKey).(response.ErrorLogInfo); ok {
		return buildResponseErrorLog(payload.StatusCode, payload.Code, payload.Message, payload.Details)
	}

	return buildResponseErrorLog(status, "HTTP_ERROR", http.StatusText(status), nil)
}

func buildResponseErrorLog(status int, code string, message string, details any) responseErrorLog {
	if status >= http.StatusInternalServerError || status == http.StatusUnauthorized || status == http.StatusForbidden {
		details = nil
	}

	return responseErrorLog{
		Status:  status,
		Code:    code,
		Message: message,
		Details: sanitizeLogValue(details, ""),
	}
}
