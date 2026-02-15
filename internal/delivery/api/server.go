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
	"radar/internal/errors"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"
	"golang.org/x/net/http2"
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
	echoServer.Server.ReadTimeout = params.Cfg.HTTP.Timeouts.ReadTimeout
	echoServer.Server.ReadHeaderTimeout = params.Cfg.HTTP.Timeouts.ReadHeaderTimeout
	echoServer.Server.WriteTimeout = params.Cfg.HTTP.Timeouts.WriteTimeout
	echoServer.Server.IdleTimeout = params.Cfg.HTTP.Timeouts.IdleTimeout

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

	// 5. Request body size limit
	echoServer.Use(echomiddleware.BodyLimit(params.Cfg.HTTP.MaxRequestBodySize))

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
	s.logger.Info("Starting API HTTP server", slog.String("host_port", hostPort))
	h2Server := &http2.Server{
		IdleTimeout: s.cfg.HTTP.Timeouts.IdleTimeout,
	}
	if err := s.server.StartH2CServer(hostPort, h2Server); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
