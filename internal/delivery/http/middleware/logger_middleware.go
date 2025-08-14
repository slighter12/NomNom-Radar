package middleware

import (
	"context"
	"log/slog"
	"time"

	"radar/config"

	"github.com/labstack/echo/v4"
)

// LoggerMiddleware controllable logging middleware
type LoggerMiddleware struct {
	logger *slog.Logger
	debug  bool
}

// NewLoggerMiddleware creates a new logger middleware
func NewLoggerMiddleware(logger *slog.Logger, config *config.Config) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger,
		debug:  config.Env.Debug,
	}
}

// Handle processes request logging
func (m *LoggerMiddleware) Handle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var err error
		if m.debug {
			start := time.Now()
			defer func() {
				m.logRequest(c, start, err)
			}()
		}

		// Execute next handler
		err = next(c)

		return err
	}
}

// logRequest logs request details
func (m *LoggerMiddleware) logRequest(c echo.Context, start time.Time, err error) {
	req := c.Request()
	res := c.Response()

	// Calculate latency
	latency := time.Since(start)

	// Prepare log fields
	fields := []slog.Attr{
		slog.String("method", req.Method),
		slog.String("uri", req.URL.Path),
		slog.Int("status", res.Status),
		slog.Duration("latency", latency),
		slog.String("latency_human", latency.String()),
		slog.String("remote_ip", c.RealIP()),
		slog.String("user_agent", req.UserAgent()),
		slog.String("time", start.Format(time.RFC3339)),
	}

	// If there are query parameters, log them too
	if len(req.URL.RawQuery) > 0 {
		fields = append(fields, slog.String("query", req.URL.RawQuery))
	}

	// If there's an error, log error details
	if err != nil {
		fields = append(fields, slog.String("error", err.Error()))
	}

	// Choose log level based on status code
	logLevel := slog.LevelInfo
	if res.Status >= 400 {
		logLevel = slog.LevelWarn
	}
	if res.Status >= 500 {
		logLevel = slog.LevelError
	}

	// Log the request
	m.logger.LogAttrs(context.Background(), logLevel, "HTTP Request", fields...)
}
