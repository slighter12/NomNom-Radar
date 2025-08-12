package postgres

import (
	"context"

	"radar/config"
	"radar/internal/domain/lifecycle"

	"github.com/pkg/errors"
	pgLib "github.com/slighter12/go-lib/database/postgres"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Params defines the required parameters
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// New creates PostgreSQL client mapping
func New(params Params) (*gorm.DB, error) {
	db, err := pgLib.New(params.Config.Postgres)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create PostgreSQL client")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get PostgreSQL sql.DB")
	}

	// Add lifecycle management
	params.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
			defer cancel()

			return sqlDB.PingContext(ctx)
		},
		OnStop: func(_ context.Context) error {
			return sqlDB.Close()
		},
	})

	return db, nil
}
