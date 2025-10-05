package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/http"
	"radar/internal/delivery/http/middleware"
	"radar/internal/delivery/http/router/handler"
	"radar/internal/domain/service"
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
			newFirebaseService,
			newQRCodeService,
		),
	)
}

// newFirebaseService creates a Firebase service with dependency injection
func newFirebaseService(ctx context.Context, cfg *config.Config, logger *slog.Logger) (service.NotificationService, error) {
	// Firebase is optional - skip if not configured
	if cfg.Firebase == nil {
		logger.Info("Firebase not configured, notification service will be disabled")

		return nil, nil
	}

	// Validate Firebase configuration
	if cfg.Firebase.ProjectID == "" || cfg.Firebase.ProjectID == "your-project-id" {
		logger.Warn("Firebase project ID not configured, notification service will be disabled")

		return nil, nil
	}

	if cfg.Firebase.CredentialsPath == "" || cfg.Firebase.CredentialsPath == "/path/to/firebase-service-account.json" {
		logger.Warn("Firebase credentials path not configured, notification service will be disabled")

		return nil, nil
	}

	svc, err := notification.NewFirebaseService(ctx, cfg.Firebase.ProjectID, cfg.Firebase.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firebase service: %w", err)
	}

	logger.Info("Firebase notification service initialized successfully")

	return svc, nil
}

// newQRCodeService creates a QR code service with dependency injection
func newQRCodeService(cfg *config.Config) service.QRCodeService {
	if cfg.QRCode == nil {
		// Use default values if not configured
		return qrcode.NewQRCodeService(256, "M")
	}

	return qrcode.NewQRCodeService(cfg.QRCode.Size, cfg.QRCode.ErrorCorrectionLevel)
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
