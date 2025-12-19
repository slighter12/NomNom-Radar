package main

import (
	"context"
	"log/slog"
	"os"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/http"
	"radar/internal/delivery/http/middleware"
	"radar/internal/delivery/http/router/handler"
	"radar/internal/infra/auth"
	"radar/internal/infra/auth/google"
	logs "radar/internal/infra/log"
	"radar/internal/infra/notification"
	"radar/internal/infra/persistence/postgres"
	"radar/internal/infra/qrcode"
	"radar/internal/usecase/impl"

	"go.uber.org/fx"
)

type startServerParams struct {
	fx.In
	fx.Lifecycle

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
			middleware.NewAuthMiddleware,
			middleware.NewErrorMiddleware,
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
				http.NewServer,
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
				os.Exit(1)
			}
		}()
	}
}
