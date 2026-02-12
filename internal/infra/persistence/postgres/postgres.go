package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"radar/config"
	"radar/internal/domain/lifecycle"
	"radar/internal/errors"

	pgLib "github.com/slighter12/go-lib/database/postgres"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	dbPoolMonitorInterval       = 5 * time.Second
	dbPoolWarnDurationThreshold = 50 * time.Millisecond
)

// Params defines the required parameters
type Params struct {
	fx.In
	fx.Lifecycle

	Config *config.Config
	Logger *slog.Logger
}

// New creates PostgreSQL client mapping
func New(params Params) (*gorm.DB, error) {
	db, err := pgLib.New(params.Config.Postgres)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create PostgreSQL client")
	}
	db = db.Session(&gorm.Session{
		// Disable GORM's per-statement implicit transaction.
		// We keep explicit transactions via txManager.Execute for multi-step atomic operations.
		SkipDefaultTransaction: true,
		Logger:                 newGormSlogLogger(params.Logger, params.Config),
	})

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get PostgreSQL sql.DB")
	}

	monitorCtx, cancelMonitor := context.WithCancel(context.Background())

	// Add lifecycle management
	params.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			ctx, cancel := context.WithTimeout(startCtx, lifecycle.DefaultTimeout)
			defer cancel()

			if err := sqlDB.PingContext(ctx); err != nil {
				return errors.Wrap(err, "failed to ping PostgreSQL")
			}

			go monitorDBPool(monitorCtx, params.Logger, sqlDB, dbPoolMonitorInterval)

			return nil
		},
		OnStop: func(_ context.Context) error {
			cancelMonitor()

			return sqlDB.Close()
		},
	})

	return db, nil
}

func monitorDBPool(ctx context.Context, logger *slog.Logger, sqlDB *sql.DB, interval time.Duration) {
	if logger == nil || sqlDB == nil {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	prev := sqlDB.Stats()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cur := sqlDB.Stats()
			waitDelta := cur.WaitCount - prev.WaitCount
			waitDurationDelta := cur.WaitDuration - prev.WaitDuration

			if waitDelta > 0 {
				attrs := []slog.Attr{
					slog.Int64("wait_count_delta", waitDelta),
					slog.Duration("wait_duration_delta", waitDurationDelta),
					slog.Duration("avg_wait", waitDurationDelta/time.Duration(waitDelta)),
					slog.Int("max_open_conns", cur.MaxOpenConnections),
					slog.Int("open_conns", cur.OpenConnections),
					slog.Int("in_use_conns", cur.InUse),
					slog.Int("idle_conns", cur.Idle),
					slog.Int64("wait_count_total", cur.WaitCount),
					slog.Duration("wait_duration_total", cur.WaitDuration),
				}
				if waitDurationDelta >= dbPoolWarnDurationThreshold {
					logger.LogAttrs(ctx, slog.LevelWarn, "Postgres pool wait detected", attrs...)
				} else {
					logger.LogAttrs(ctx, slog.LevelDebug, "Postgres pool wait observed", attrs...)
				}
			}

			prev = cur
		}
	}
}
