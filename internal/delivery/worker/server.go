package worker

import (
	"context"
	"errors"
	"fmt"
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

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"go.uber.org/fx"
)

type workerServer struct {
	cfg    *config.Config
	logger *slog.Logger
	server *echo.Echo
	// httpServer owns the listener lifecycle; Echo v5 no longer exposes one.
	httpServer *http.Server
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

	// 4. Convert returned errors and inner panics into HTTP responses inside the middleware chain
	e.Use(errorMiddleware.HandleErrors)
	e.Use(errorMiddleware.RecoverPanics)

	// Health check endpoint
	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Pub/Sub push endpoint
	e.POST("/push", params.PushHandler.HandlePush)

	srv := &workerServer{
		cfg:        params.Cfg,
		logger:     params.Logger,
		server:     e,
		httpServer: newHTTPServer(e, params.Cfg, params.Logger),
	}

	params.Lc.Append(fx.Hook{
		OnStop: srv.stop,
	})

	return srv, nil
}

// newHTTPServer builds the standard library server that fronts Echo. It serves
// HTTP/1 and unencrypted HTTP/2 (h2c) on the same port via the standard
// library, avoiding the deprecated golang.org/x/net/http2 API.
func newHTTPServer(handler http.Handler, cfg *config.Config, logger *slog.Logger) *http.Server {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	return &http.Server{
		Handler:           handler,
		ReadTimeout:       cfg.HTTP.Timeouts.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.Timeouts.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.Timeouts.WriteTimeout,
		IdleTimeout:       cfg.HTTP.Timeouts.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		Protocols:         &protocols,
	}
}

// Serve starts the worker HTTP server
func (s *workerServer) Serve(ctx context.Context) error {
	hostPort := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.cfg.HTTP.Port))
	s.logger.Info("Starting Worker HTTP server", slog.String("host_port", hostPort))

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", hostPort)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", hostPort, err)
	}

	if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve %s HTTP: %w", "'+name+'", err)
	}

	return nil
}

// stop gracefully shuts down the worker server
func (s *workerServer) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down Worker HTTP server")

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown %s HTTP server: %w", "'+name+'", err)
	}

	return nil
}
