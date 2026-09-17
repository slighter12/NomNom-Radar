package worker

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"radar/config"
	"radar/internal/delivery"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/middleware"
	"radar/internal/delivery/worker/handler"
	"radar/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"
)

type workerServer struct {
	cfg    *config.Config
	logger *slog.Logger
	server *echo.Echo
}

// ServerParams holds dependencies for the worker server
type ServerParams struct {
	fx.In

	Lc          fx.Lifecycle
	Cfg         *config.Config
	Logger      *slog.Logger
	PushHandler *handler.PushHandler
}

// NewServer creates a new worker HTTP server
func NewServer(params ServerParams) (delivery.Delivery, error) {
	e := echo.New()
	e.HideBanner = true
	e.Server.ReadTimeout = params.Cfg.HTTP.Timeouts.ReadTimeout
	e.Server.ReadHeaderTimeout = params.Cfg.HTTP.Timeouts.ReadHeaderTimeout
	e.Server.WriteTimeout = params.Cfg.HTTP.Timeouts.WriteTimeout
	e.Server.IdleTimeout = params.Cfg.HTTP.Timeouts.IdleTimeout

	// Set up middleware in correct order
	// 1. Recover middleware first (to catch panics early)
	e.Use(echomiddleware.Recover())

	// 2. Request ID middleware (must be before logger to include in logs)
	requestIDMiddleware := middleware.NewRequestIDMiddleware(params.Logger)
	e.Use(requestIDMiddleware.Process)

	// Set up centralized error handler
	errorMiddleware := apimiddleware.NewErrorMiddleware(params.Logger)
	e.HTTPErrorHandler = errorMiddleware.HandleHTTPError

	// 3. Request lifecycle logger
	requestLogger := apimiddleware.NewRequestLoggerMiddleware(params.Logger, params.Cfg)
	e.Use(requestLogger.Log)

	// 4. Convert returned errors into HTTP responses inside the middleware chain
	e.Use(errorMiddleware.HandleErrors)

	// 5. Keep a bounded JSON body copy for sanitized error-only request logging
	e.Use(apimiddleware.CaptureRequestBodyForErrorLog)

	// Health check endpoint
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Pub/Sub push endpoint
	e.POST("/push", params.PushHandler.HandlePush)

	srv := &workerServer{
		cfg:    params.Cfg,
		logger: params.Logger,
		server: e,
	}

	params.Lc.Append(fx.Hook{
		OnStop: srv.stop,
	})

	return srv, nil
}

// Serve starts the worker HTTP server
func (s *workerServer) Serve(ctx context.Context) error {
	hostPort := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.cfg.HTTP.Port))
	s.logger.Info("Starting Worker HTTP server", slog.String("host_port", hostPort))

	// Serve HTTP/1 and unencrypted HTTP/2 (h2c) on the same port via the
	// standard library, avoiding the deprecated golang.org/x/net/http2 API.
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	s.server.Server.Protocols = &protocols

	if err := s.server.Start(hostPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// stop gracefully shuts down the worker server
func (s *workerServer) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down Worker HTTP server")

	return s.server.Shutdown(shutdownCtx)
}
