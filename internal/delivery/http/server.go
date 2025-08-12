package http

import (
	"context"
	"log/slog"
	"net"
	"strconv"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/http/router"
	"radar/internal/delivery/http/validator"
	"radar/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	slogecho "github.com/samber/slog-echo"
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
	echoServer.Use(slogecho.New(params.Logger))
	echoServer.Validator = validator.New()
	echoServer.Use(middleware.Recover())
	echoServer.Use(middleware.CORS())

	router := router.NewRouter(params.RouterParams)
	router.RegisterRoutes(echoServer)

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
