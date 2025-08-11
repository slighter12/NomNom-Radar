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

// Params 定義所需的參數
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
}

// New 創建 PostgreSQL 客戶端映射
func New(params Params) (*gorm.DB, error) {
	db, err := pgLib.New(params.Config.Postgres)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create PostgreSQL client")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get PostgreSQL sql.DB")
	}

	// 添加生命週期管理
	params.Lifecycle.Append(fx.Hook{
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
