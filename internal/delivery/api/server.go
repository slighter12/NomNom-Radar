package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"radar/config"
	"radar/internal/delivery"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router"
	"radar/internal/delivery/api/validator"
	"radar/internal/delivery/middleware"
	"radar/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

type apiServer struct {
	cfg    *config.Config
	logger *slog.Logger
	server *echo.Echo
}

// ServerParams holds dependencies for HTTP server, injected by Fx.
type ServerParams struct {
	fx.In

	Lc           fx.Lifecycle
	Cfg          *config.Config
	Logger       *slog.Logger
	RouterParams router.RouterParams
}

func NewServer(params ServerParams) (delivery.Delivery, error) {
	echoServer := echo.New()
	echoServer.HideBanner = true

	// Set up middleware in correct order
	// 1. Recover middleware first (to catch panics early)
	echoServer.Use(echomiddleware.Recover())

	// 2. Request ID middleware (must be before logger to include in logs)
	requestIDMiddleware := middleware.NewRequestIDMiddleware(params.Logger)
	echoServer.Use(requestIDMiddleware.Process)

	// 3. Logger middleware
	loggerMiddleware := middleware.NewLoggerMiddleware(params.Logger, params.Cfg)
	echoServer.Use(loggerMiddleware.Handle)

	// 4. CORS middleware
	echoServer.Use(echomiddleware.CORS())

	// Set up centralized error handler
	errorMiddleware := apimiddleware.NewErrorMiddleware(params.Logger)
	echoServer.HTTPErrorHandler = errorMiddleware.HandleHTTPError

	// Set up validator
	echoServer.Validator = validator.New()

	r := router.NewRouter(params.RouterParams)
	r.RegisterRoutes(echoServer)
	r.RegisterTestRoutes(echoServer)

	srv := &apiServer{
		cfg:    params.Cfg,
		logger: params.Logger,
		server: echoServer,
	}

	params.Lc.Append(fx.Hook{
		OnStop: srv.stop,
	})

	return srv, nil
}

func (s *apiServer) Serve(ctx context.Context) error {
	hostPort := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.cfg.HTTP.Port))
	s.logger.Info("Starting API HTTP server", slog.String("hostPort", hostPort))
	if err := s.server.Start(hostPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.WithStack(err)
	}

	return nil
}

func (s *apiServer) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down API HTTP server")

	return errors.WithStack(s.server.Shutdown(shutdownCtx))
}
