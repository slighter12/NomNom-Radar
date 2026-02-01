package main

import (
	"context"
	"log/slog"
	"os"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/api"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router/handler"
	"radar/internal/infra/auth"
	"radar/internal/infra/auth/google"
	logs "radar/internal/infra/log"
	"radar/internal/infra/notification"
	"radar/internal/infra/persistence/postgres"
	"radar/internal/infra/pubsub"
	"radar/internal/infra/qrcode"
	"radar/internal/usecase/impl"

	"go.uber.org/fx"
)

type startServerParams struct {
	fx.In
	fx.Lifecycle
	fx.Shutdowner

	Deliveries []delivery.Delivery `group:"deliveries"`
}

func main() {
	fx.New(
		injectInfra(),
		injectRepo(),
		injectService(),
		injectUsecase(),
		injectDelivery(),
		injectMiddleware(),
		injectHandler(),
		fx.Invoke(
			startServer,
		),
	).Run()
}

func injectInfra() fx.Option {
	return fx.Provide(
		config.New,
		// Expose routing config as a separate dependency to satisfy RoutingUsecase
		func(cfg *config.Config) *config.RoutingConfig {
			if cfg == nil || cfg.Routing == nil {
				// Provide an empty config to keep Fx wiring intact even if the section is missing
				return &config.RoutingConfig{}
			}

			return cfg.Routing
		},
		logs.New,
		context.Background,
		postgres.New,
	)
}

func injectRepo() fx.Option {
	return fx.Options(
		fx.Provide(
			postgres.NewUserRepository,
			postgres.NewAuthRepository,
			postgres.NewAddressRepository,
			postgres.NewRefreshTokenRepository,
			postgres.NewTransactionManager,
			postgres.NewDeviceRepository,
			postgres.NewSubscriptionRepository,
			postgres.NewNotificationRepository,
		),
	)
}

func injectService() fx.Option {
	return fx.Options(
		fx.Provide(
			auth.NewBcryptHasher,
			auth.NewJWTService,
			google.NewOAuthService,
			notification.NewFirebaseService,
			qrcode.NewQRCodeService,
			pubsub.NewEventPublisher,
		),
	)
}

func injectUsecase() fx.Option {
	return fx.Options(
		fx.Provide(
			impl.NewUserService,
			impl.NewProfileService,
			impl.NewSessionService,
			impl.NewLocationService,
			impl.NewDeviceService,
			impl.NewSubscriptionService,
			impl.NewNotificationService,
			impl.NewRoutingService,
		),
	)
}

func injectMiddleware() fx.Option {
	return fx.Options(
		fx.Provide(
			apimiddleware.NewAuthMiddleware,
			apimiddleware.NewErrorMiddleware,
		),
	)
}

func injectHandler() fx.Option {
	return fx.Options(
		fx.Provide(
			handler.NewUserHandler,
			handler.NewTestHandler,
			handler.NewLocationHandler,
			handler.NewDeviceHandler,
			handler.NewSubscriptionHandler,
			handler.NewNotificationHandler,
		),
	)
}

func injectDelivery() fx.Option {
	return fx.Options(
		fx.Provide(
			fx.Annotate(
				api.NewServer,
				fx.ResultTags(`group:"deliveries"`),
			),
		),
	)
}

func startServer(ctx context.Context, params startServerParams) {
	for _, delivery := range params.Deliveries {
		go func() {
			if err := delivery.Serve(ctx); err != nil {
				slog.Error("Failed to start server", slog.Any("error", err))

				// Trigger graceful shutdown to execute all OnStop hooks
				if shutdownErr := params.Shutdown(); shutdownErr != nil {
					slog.Error("Failed to shutdown gracefully", slog.Any("error", shutdownErr))
					os.Exit(1)
				}
			}
		}()
	}
}
