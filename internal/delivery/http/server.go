package http

import (
	"context"
	"log/slog"
	"net"
	"strconv"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/http/middleware"
	"radar/internal/delivery/http/router"
	"radar/internal/delivery/http/validator"
	"radar/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

type HTTPParams struct {
	fx.In
	fx.Lifecycle

	Config       *config.Config
	Logger       *slog.Logger
	RouterParams router.RouterParams
}

type httpServer struct {
	cfg    *config.Config
	logger *slog.Logger
	server *echo.Echo
}

func NewServer(params HTTPParams) (delivery.Delivery, error) {
	echoServer := echo.New()
	echoServer.HideBanner = true

	// Set up middleware in correct order
	// 1. Recover middleware first (to catch panics early)
	echoServer.Use(echomiddleware.Recover())

	// 2. Logger middleware
	loggerMiddleware := middleware.NewLoggerMiddleware(params.Logger, params.Config)
	echoServer.Use(loggerMiddleware.Handle)

	// 3. CORS middleware
	echoServer.Use(echomiddleware.CORS())

	// Set up centralized error handler
	errorMiddleware := middleware.NewErrorMiddleware(params.Logger)
	echoServer.HTTPErrorHandler = errorMiddleware.HandleHTTPError

	// Set up validator
	echoServer.Validator = validator.New()

	router := router.NewRouter(params.RouterParams)
	router.RegisterRoutes(echoServer)
	router.RegisterTestRoutes(echoServer)

	delivery := &httpServer{
		cfg:    params.Config,
		logger: params.Logger,
		server: echoServer,
	}

	params.Append(fx.Hook{
		OnStop: delivery.stop,
	})

	return delivery, nil
}

func (s *httpServer) Serve(ctx context.Context) error {
	hostPort := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.cfg.HTTP.Port))
	s.logger.Info("Starting HTTP server", slog.String("hostPort", hostPort))
	if err := s.server.Start(hostPort); err != nil {
		return errors.Wrap(err, "failed to serve https")
	}

	return nil
}

func (s *httpServer) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down HTTP server")

	return errors.WithStack(s.server.Shutdown(shutdownCtx))
}
