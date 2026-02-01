package main

import (
	"context"
	"log/slog"
	"os"

	"radar/config"
	"radar/internal/delivery"
	"radar/internal/delivery/worker"
	"radar/internal/delivery/worker/handler"
	logs "radar/internal/infra/log"
	"radar/internal/infra/notification"
	"radar/internal/infra/persistence/postgres"
	"radar/internal/infra/routing/pmtiles"

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
		injectHandler(),
		injectDelivery(),
		fx.Invoke(
			startServer,
		),
	).Run()
}

func injectInfra() fx.Option {
	return fx.Provide(
		config.New,
		// Expose PMTiles config for the routing service
		func(cfg *config.Config) *config.PMTilesConfig {
			if cfg == nil || cfg.PMTiles == nil {
				return &config.PMTilesConfig{}
			}

			return cfg.PMTiles
		},
		logs.New,
		context.Background,
		postgres.New,
	)
}

func injectRepo() fx.Option {
	return fx.Options(
		fx.Provide(
			postgres.NewSubscriptionRepository,
			postgres.NewDeviceRepository,
			postgres.NewNotificationRepository,
		),
	)
}

func injectService() fx.Option {
	return fx.Options(
		fx.Provide(
			notification.NewFirebaseService,
			pmtiles.NewPMTilesRoutingService,
		),
	)
}

func injectHandler() fx.Option {
	return fx.Options(
		fx.Provide(
			handler.NewPushHandler,
		),
	)
}

func injectDelivery() fx.Option {
	return fx.Options(
		fx.Provide(
			fx.Annotate(
				worker.NewServer,
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
