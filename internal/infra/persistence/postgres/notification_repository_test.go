package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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

	db, sqlLogger := newDryRunDB(t)
	repo, ok := NewNotificationRepository(db).(*notificationRepository)
	require.True(t, ok)

	return &dryRunNotificationRepository{repo: repo, sqlLogger: sqlLogger}
}

func notificationListSQL(dryRun *dryRunNotificationRepository, merchantID uuid.UUID, limit, offset int) string {
	return capturedSQL(dryRun.sqlLogger, func() {
		_, _ = dryRun.repo.FindNotificationsByMerchant(context.Background(), merchantID, limit, offset)
	})
}
