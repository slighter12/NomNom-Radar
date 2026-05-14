package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

type DiscoveryRepository interface {
	FindCategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoveryCategory, error)
	FindCategoryBySlug(ctx context.Context, slug string) (*entity.DiscoveryCategory, error)
	FindSubcategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoverySubcategory, error)
	FindSubcategoryBySlug(ctx context.Context, slug string) (*entity.DiscoverySubcategory, error)
	FindHubByID(ctx context.Context, id uuid.UUID) (*entity.Hub, error)
	FindHubBySlug(ctx context.Context, slug string) (*entity.Hub, error)
	ListActiveCategories(ctx context.Context) ([]*entity.DiscoveryCategory, error)
	ListActiveSubcategories(ctx context.Context) ([]*entity.DiscoverySubcategory, error)
	ListActiveHubs(ctx context.Context) ([]*entity.Hub, error)
}
