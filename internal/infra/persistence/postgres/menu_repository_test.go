package postgres

import (
	"context"
	"testing"

	"radar/internal/domain/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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

	db, sqlLogger := newDryRunDB(t)
	repo, ok := NewMenuRepository(db).(*menuRepository)
	require.True(t, ok)

	return &dryRunMenuRepository{repo: repo, sqlLogger: sqlLogger}
}

func menuListSQL(dryRun *dryRunMenuRepository, filter repository.MenuItemListFilter) string {
	return capturedSQL(dryRun.sqlLogger, func() {
		_, _, _ = dryRun.repo.ListMenuItemsByMerchant(context.Background(), uuid.New(), filter)
	})
}
