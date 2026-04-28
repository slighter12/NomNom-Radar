package main

import (
	"context"
	"fmt"
	"log/slog"

	"radar/config"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	logs "radar/internal/infra/log"
	"radar/internal/infra/persistence/postgres"

	"go.uber.org/fx"
)

type cleanupParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Shutdown  fx.Shutdowner

	DeviceRepo repository.DeviceRepository
	Config     *config.Config
	Logger     *slog.Logger
}

func main() {
	fx.New(
		injectInfra(),
		injectRepo(),
		fx.Invoke(runDeviceCleanup),
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
	return fx.Provide(postgres.NewDeviceRepository)
}

func runDeviceCleanup(params cleanupParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			cleanupCtx, cancel := context.WithTimeout(ctx, params.Config.DeviceCleanup.Timeout)
			defer cancel()

			rowsAffected, err := params.DeviceRepo.SoftDeleteStaleDevices(cleanupCtx, policy.DefaultDevicePolicy().StaleCleanupDays)
			if err != nil {
				return fmt.Errorf("soft delete stale devices: %w", err)
			}

			params.Logger.Info(
				"Device cleanup completed",
				slog.Int("stale_days", policy.DefaultDevicePolicy().StaleCleanupDays),
				slog.Int64("rows_affected", rowsAffected),
			)

			return params.Shutdown.Shutdown()
		},
	})
}
