package http

import (
	"context"
	"fmt"
	"log/slog"

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

	params.Lifecycle.Append(fx.Hook{
		OnStop: delivery.stop,
	})

	return delivery, nil
}

func (s *httpServer) Serve(ctx context.Context) error {
	s.logger.Info("Starting HTTP server", slog.Int("port", s.cfg.HTTP.Port))
	if err := s.server.Start(fmt.Sprintf(":%d", s.cfg.HTTP.Port)); err != nil {
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
