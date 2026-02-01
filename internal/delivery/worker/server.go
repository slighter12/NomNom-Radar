package worker

import (
	"context"
	"log/slog"
	"net"
	"strconv"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/middleware"
	"radar/internal/delivery/worker/handler"
	"radar/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
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

	// Set up middleware in correct order
	// 1. Recover middleware first (to catch panics early)
	e.Use(echomiddleware.Recover())

	// 2. Request ID middleware (must be before logger to include in logs)
	requestIDMiddleware := middleware.NewRequestIDMiddleware(params.Logger)
	e.Use(requestIDMiddleware.Process)

	// 3. Logger middleware
	loggerMiddleware := middleware.NewLoggerMiddleware(params.Logger, params.Cfg)
	e.Use(loggerMiddleware.Handle)

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
	s.logger.Info("Starting Worker HTTP server", slog.String("hostPort", hostPort))
	if err := s.server.Start(hostPort); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// stop gracefully shuts down the worker server
func (s *workerServer) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down Worker HTTP server")

	return errors.WithStack(s.server.Shutdown(shutdownCtx))
}
