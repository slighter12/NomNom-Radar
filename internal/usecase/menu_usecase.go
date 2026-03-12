package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

type ListMerchantMenuItemsInput struct {
	Category    string
	IsAvailable *bool
	Keyword     string
	Page        int
	PageSize    int
}

type CreateMenuItemInput struct {
	Name        string
	Description *string
	Category    string
	Price       int
	Currency    string
	PrepMinutes int
	IsAvailable *bool
	IsPopular   *bool
	ImageURL    *string
	ExternalURL *string
}

type UpdateMenuItemInput struct {
	Name        string
	Description *string
	Category    string
	Price       int
	Currency    string
	PrepMinutes int
	IsAvailable bool
	IsPopular   bool
	ImageURL    *string
	ExternalURL *string
}

type ReorderMenuItemsInput struct {
	ItemIDs []uuid.UUID `json:"item_ids"`
}

type MerchantMenuItemsResult struct {
	Items      []*entity.MenuItem      `json:"items"`
	Pagination *MerchantMenuPagination `json:"pagination"`
}

type MerchantMenuPagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type ReorderMenuItemsResult struct {
	UpdatedCount int `json:"updated_count"`
}

type MenuUsecase interface {
	GetPublicMerchantMenu(ctx context.Context, merchantID uuid.UUID, input *ListMerchantMenuItemsInput) (*MerchantMenuItemsResult, error)
	ListMerchantMenuItems(ctx context.Context, merchantID uuid.UUID, input *ListMerchantMenuItemsInput) (*MerchantMenuItemsResult, error)
	CreateMenuItem(ctx context.Context, merchantID uuid.UUID, input *CreateMenuItemInput) (*entity.MenuItem, error)
	UpdateMenuItem(ctx context.Context, merchantID, itemID uuid.UUID, input *UpdateMenuItemInput) (*entity.MenuItem, error)
	UpdateMenuItemStatus(ctx context.Context, merchantID, itemID uuid.UUID, isAvailable bool) (*entity.MenuItem, error)
	ReorderMenuItems(ctx context.Context, merchantID uuid.UUID, input *ReorderMenuItemsInput) (*ReorderMenuItemsResult, error)
	DeleteMenuItem(ctx context.Context, merchantID, itemID uuid.UUID) error
}
