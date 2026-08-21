package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNotificationRepository_FindNotificationsByMerchantQuery_UsesStableOrdering(t *testing.T) {
	repo := newDryRunNotificationRepository(t)

	sql := notificationListSQL(repo, uuid.New(), 20, 20)

	require.Contains(t, sql, "ORDER BY merchant_location_notifications.published_at DESC,merchant_location_notifications.id ASC")
}

type dryRunNotificationRepository struct {
	repo      *notificationRepository
	sqlLogger *captureSQLLogger
}

func newDryRunNotificationRepository(t *testing.T) *dryRunNotificationRepository {
	t.Helper()

	sqlLogger := &captureSQLLogger{}
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=localhost user=test password=test dbname=test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: sqlLogger})
	require.NoError(t, err)

	repo, ok := NewNotificationRepository(db).(*notificationRepository)
	require.True(t, ok)

	return &dryRunNotificationRepository{repo: repo, sqlLogger: sqlLogger}
}

func notificationListSQL(dryRun *dryRunNotificationRepository, merchantID uuid.UUID, limit, offset int) string {
	dryRun.sqlLogger.queries = nil

	_, _ = dryRun.repo.FindNotificationsByMerchant(context.Background(), merchantID, limit, offset)

	sql := strings.Join(dryRun.sqlLogger.queries, " ")
	sql = strings.ReplaceAll(sql, `"`, "")

	return strings.Join(strings.Fields(sql), " ")
}
