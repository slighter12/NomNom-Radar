package repository

import (
	"context"

	"radar/internal/domain/entity"
	"radar/internal/errors"

	"github.com/google/uuid"
)

var ErrMenuItemNotFound = errors.New("menu item not found")

type MenuItemListFilter struct {
	Category    *entity.MenuCategory
	IsAvailable *bool
	Keyword     string
	Limit       int
	Offset      int
}

type MenuRepository interface {
	CreateMenuItem(ctx context.Context, item *entity.MenuItem) error
	FindMenuItemByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error)
	ListActiveMenuItemIDsByMerchant(ctx context.Context, merchantID uuid.UUID) ([]uuid.UUID, error)
	ListMenuItemsByMerchant(ctx context.Context, merchantID uuid.UUID, filter MenuItemListFilter) ([]*entity.MenuItem, int64, error)
	UpdateMenuItem(ctx context.Context, item *entity.MenuItem) error
	UpdateMenuItemAvailability(ctx context.Context, id uuid.UUID, isAvailable bool) error
	DeleteMenuItem(ctx context.Context, merchantID, id uuid.UUID) error
	ReorderMenuItems(ctx context.Context, merchantID uuid.UUID, itemIDs []uuid.UUID) error
}
