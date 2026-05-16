package usecase

import (
	"context"
	"time"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

type DiscoveryFilterRef struct {
	ID   *uuid.UUID
	Slug string
}

type ListDiscoveryCategoriesResult struct {
	Categories []*DiscoveryCategoryResult `json:"categories"`
}

type DiscoveryCategoryResult struct {
	ID            uuid.UUID                     `json:"id"`
	Slug          string                        `json:"slug"`
	Name          string                        `json:"name"`
	DisplayOrder  int                           `json:"display_order"`
	Subcategories []*DiscoverySubcategoryResult `json:"subcategories"`
}

type DiscoverySubcategoryResult struct {
	ID           uuid.UUID `json:"id"`
	CategoryID   uuid.UUID `json:"category_id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
}

type ListDiscoverySubcategoriesResult struct {
	Subcategories []*DiscoverySubcategoryResult `json:"subcategories"`
}

type ListDiscoveryHubsResult struct {
	Hubs []*DiscoveryHubResult `json:"hubs"`
}

type DiscoveryHubResult struct {
	ID       uuid.UUID      `json:"id"`
	Slug     string         `json:"slug"`
	Name     string         `json:"name"`
	Type     entity.HubType `json:"type"`
	City     string         `json:"city"`
	AreaName string         `json:"area_name"`
	StartsAt *time.Time     `json:"starts_at,omitempty"`
	EndsAt   *time.Time     `json:"ends_at,omitempty"`
}

type SearchPublicMerchantsInput struct {
	Keyword      string
	Category     DiscoveryFilterRef
	Subcategory  DiscoveryFilterRef
	Hub          DiscoveryFilterRef
	Latitude     *float64
	Longitude    *float64
	RadiusMeters *int
	Page         int
	PageSize     int
}

type SearchPublicMerchantsResult struct {
	Merchants  []*entity.PublicMerchantSearchItem `json:"merchants"`
	Pagination *MerchantSearchPagination          `json:"pagination"`
}

type MerchantSearchPagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type DiscoveryUsecase interface {
	ListActiveCategories(ctx context.Context) (*ListDiscoveryCategoriesResult, error)
	ListActiveSubcategories(ctx context.Context) (*ListDiscoverySubcategoriesResult, error)
	ListActiveHubs(ctx context.Context) (*ListDiscoveryHubsResult, error)
	SearchPublicMerchants(
		ctx context.Context,
		input *SearchPublicMerchantsInput,
	) (*SearchPublicMerchantsResult, error)
}
