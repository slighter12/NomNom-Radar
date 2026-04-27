package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDeleteExpiredRefreshTokensQuery_UsesExplicitRevokedRetentionGrouping(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=localhost user=test password=test dbname=test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	require.NoError(t, err)

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	revokedCutoff := now.AddDate(0, 0, -7)

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return deleteExpiredRefreshTokensQuery(tx, now, revokedCutoff)
	})
	sql = strings.Join(strings.Fields(sql), " ")

	require.Contains(t, sql, "expires_at <")
	require.Contains(t, sql, "OR (is_revoked =")
	require.Contains(t, sql, "AND created_at <")
}
