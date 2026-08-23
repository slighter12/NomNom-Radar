package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type captureSQLLogger struct {
	queries []string
}

func (capture *captureSQLLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return capture
}

func (*captureSQLLogger) Info(context.Context, string, ...any) {
}

func (*captureSQLLogger) Warn(context.Context, string, ...any) {
}

func (*captureSQLLogger) Error(context.Context, string, ...any) {
}

func (capture *captureSQLLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	capture.queries = append(capture.queries, sql)
}

func newDryRunDB(t *testing.T) (*gorm.DB, *captureSQLLogger) {
	t.Helper()

	sqlLogger := &captureSQLLogger{}
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=localhost user=test password=test dbname=test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: sqlLogger})
	require.NoError(t, err)

	return db, sqlLogger
}

func capturedSQL(logger *captureSQLLogger, run func()) string {
	logger.queries = nil

	run()

	sql := strings.Join(logger.queries, " ")
	sql = strings.ReplaceAll(sql, `"`, "")

	return strings.Join(strings.Fields(sql), " ")
}
