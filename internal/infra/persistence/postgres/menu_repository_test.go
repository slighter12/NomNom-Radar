package postgres

import (
	"context"
	"strings"
	"testing"

	"radar/internal/domain/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMenuRepository_ListMenuItemsByMerchantQuery_UsesStableOrdering(t *testing.T) {
	repo := newDryRunMenuRepository(t)

	sql := menuListSQL(repo, repository.MenuItemListFilter{
		Limit:  20,
		Offset: 20,
	})

	require.Contains(t, sql, "ORDER BY menu_items.display_order ASC,menu_items.created_at ASC,menu_items.id ASC")
}

type dryRunMenuRepository struct {
	repo      *menuRepository
	sqlLogger *captureSQLLogger
}

func newDryRunMenuRepository(t *testing.T) *dryRunMenuRepository {
	t.Helper()

	sqlLogger := &captureSQLLogger{}
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=localhost user=test password=test dbname=test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: sqlLogger})
	require.NoError(t, err)

	repo, ok := NewMenuRepository(db).(*menuRepository)
	require.True(t, ok)

	return &dryRunMenuRepository{repo: repo, sqlLogger: sqlLogger}
}

func menuListSQL(dryRun *dryRunMenuRepository, filter repository.MenuItemListFilter) string {
	dryRun.sqlLogger.queries = nil

	_, _, _ = dryRun.repo.ListMenuItemsByMerchant(context.Background(), uuid.New(), filter)

	sql := strings.Join(dryRun.sqlLogger.queries, " ")
	sql = strings.ReplaceAll(sql, `"`, "")

	return strings.Join(strings.Fields(sql), " ")
}
