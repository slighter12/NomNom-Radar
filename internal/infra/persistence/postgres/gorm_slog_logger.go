package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"radar/config"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const defaultGormSlowThreshold = 200 * time.Millisecond

type gormSlogLogger struct {
	logger                     *slog.Logger
	level                      logger.LogLevel
	slowThreshold              time.Duration
	ignoreRecordNotFoundErrors bool
}

func newGormSlogLogger(baseLogger *slog.Logger, cfg *config.Config) logger.Interface {
	level := logger.Warn
	if cfg != nil && cfg.Env.Debug {
		level = logger.Info
	}

	return &gormSlogLogger{
		logger:                     baseLogger,
		level:                      level,
		slowThreshold:              defaultGormSlowThreshold,
		ignoreRecordNotFoundErrors: true,
	}
}

func (l *gormSlogLogger) LogMode(level logger.LogLevel) logger.Interface {
	cloned := *l
	cloned.level = level

	return &cloned
}

func (l *gormSlogLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level < logger.Info || l.logger == nil {
		return
	}

	l.logger.LogAttrs(ctx, slog.LevelInfo, "GORM info",
		slog.String("message", fmt.Sprintf(msg, args...)),
	)
}

func (l *gormSlogLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level < logger.Warn || l.logger == nil {
		return
	}

	l.logger.LogAttrs(ctx, slog.LevelWarn, "GORM warn",
		slog.String("message", fmt.Sprintf(msg, args...)),
	)
}

func (l *gormSlogLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level < logger.Error || l.logger == nil {
		return
	}

	l.logger.LogAttrs(ctx, slog.LevelError, "GORM error",
		slog.String("message", fmt.Sprintf(msg, args...)),
	)
}

func (l *gormSlogLogger) Trace(ctx context.Context, begin time.Time, sqlAndRowsFn func() (string, int64), err error) {
	if l.logger == nil || l.level == logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	if l.shouldLogError(err) {
		baseAttrs := l.buildQueryAttrs(sqlAndRowsFn, elapsed)
		attrs := slices.Clone(baseAttrs)
		attrs = append(attrs, slog.String("error", err.Error()))
		l.logger.LogAttrs(ctx, slog.LevelError, "GORM query failed", attrs...)

		return
	}

	if l.shouldLogSlow(elapsed) {
		baseAttrs := l.buildQueryAttrs(sqlAndRowsFn, elapsed)
		attrs := slices.Clone(baseAttrs)
		attrs = append(attrs, slog.Duration("slowThreshold", l.slowThreshold))
		l.logger.LogAttrs(ctx, slog.LevelWarn, "GORM slow query", attrs...)

		return
	}

	if l.level >= logger.Info {
		baseAttrs := l.buildQueryAttrs(sqlAndRowsFn, elapsed)
		l.logger.LogAttrs(ctx, slog.LevelInfo, "GORM query", baseAttrs...)
	}
}

func (l *gormSlogLogger) buildQueryAttrs(sqlAndRowsFn func() (string, int64), elapsed time.Duration) []slog.Attr {
	sql, rows := sqlAndRowsFn()

	return []slog.Attr{
		slog.Duration("elapsed", elapsed),
		slog.Int64("rows", rows),
		slog.String("sql", sql),
	}
}

func (l *gormSlogLogger) shouldLogError(err error) bool {
	if err == nil || l.level < logger.Error {
		return false
	}

	if l.ignoreRecordNotFoundErrors && errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}

	return true
}

func (l *gormSlogLogger) shouldLogSlow(elapsed time.Duration) bool {
	return l.slowThreshold > 0 && elapsed > l.slowThreshold && l.level >= logger.Warn
}
