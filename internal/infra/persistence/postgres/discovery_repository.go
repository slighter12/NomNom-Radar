package postgres

import (
	"context"
	"errors"
	"strings"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type discoveryRepository struct {
	q *query.Query
}

func NewDiscoveryRepository(db *gorm.DB) repository.DiscoveryRepository {
	return &discoveryRepository{q: query.Use(db)}
}

func (repo *discoveryRepository) FindCategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoveryCategory, error) {
	category, err := repo.q.DiscoveryCategoryModel.WithContext(ctx).
		Where(repo.q.DiscoveryCategoryModel.ID.Eq(id)).
		First()
	if err != nil {
		return nil, discoveryCategoryLookupError(err)
	}

	return toDiscoveryCategoryDomain(category), nil
}

func (repo *discoveryRepository) FindCategoryBySlug(ctx context.Context, slug string) (*entity.DiscoveryCategory, error) {
	category, err := repo.q.DiscoveryCategoryModel.WithContext(ctx).
		Where(repo.q.DiscoveryCategoryModel.Slug.Eq(normalizeDiscoverySlug(slug))).
		First()
	if err != nil {
		return nil, discoveryCategoryLookupError(err)
	}

	return toDiscoveryCategoryDomain(category), nil
}

func (repo *discoveryRepository) FindSubcategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoverySubcategory, error) {
	subcategory, err := repo.q.DiscoverySubcategoryModel.WithContext(ctx).
		Where(repo.q.DiscoverySubcategoryModel.ID.Eq(id)).
		First()
	if err != nil {
		return nil, discoverySubcategoryLookupError(err)
	}

	return toDiscoverySubcategoryDomain(subcategory), nil
}

func (repo *discoveryRepository) FindSubcategoryBySlug(ctx context.Context, slug string) (*entity.DiscoverySubcategory, error) {
	subcategory, err := repo.q.DiscoverySubcategoryModel.WithContext(ctx).
		Where(repo.q.DiscoverySubcategoryModel.Slug.Eq(normalizeDiscoverySlug(slug))).
		First()
	if err != nil {
		return nil, discoverySubcategoryLookupError(err)
	}

	return toDiscoverySubcategoryDomain(subcategory), nil
}

func (repo *discoveryRepository) FindHubByID(ctx context.Context, id uuid.UUID) (*entity.Hub, error) {
	hub, err := repo.q.HubModel.WithContext(ctx).
		Where(repo.q.HubModel.ID.Eq(id)).
		First()
	if err != nil {
		return nil, hubLookupError(err)
	}

	return toHubDomain(hub), nil
}

func (repo *discoveryRepository) FindHubBySlug(ctx context.Context, slug string) (*entity.Hub, error) {
	hub, err := repo.q.HubModel.WithContext(ctx).
		Where(repo.q.HubModel.Slug.Eq(normalizeDiscoverySlug(slug))).
		First()
	if err != nil {
		return nil, hubLookupError(err)
	}

	return toHubDomain(hub), nil
}

func (repo *discoveryRepository) ListActiveCategories(ctx context.Context) ([]*entity.DiscoveryCategory, error) {
	categories, err := repo.q.DiscoveryCategoryModel.WithContext(ctx).
		Where(repo.q.DiscoveryCategoryModel.Status.Eq(string(entity.DiscoveryStatusActive))).
		Order(repo.q.DiscoveryCategoryModel.DisplayOrder.Asc(), repo.q.DiscoveryCategoryModel.Name.Asc()).
		Find()
	if err != nil {
		return nil, domainerrors.ErrPersistenceFailed
	}

	result := make([]*entity.DiscoveryCategory, 0, len(categories))
	for _, category := range categories {
		result = append(result, toDiscoveryCategoryDomain(category))
	}

	return result, nil
}

func (repo *discoveryRepository) ListActiveSubcategories(ctx context.Context) ([]*entity.DiscoverySubcategory, error) {
	subcategories, err := repo.q.DiscoverySubcategoryModel.WithContext(ctx).
		Where(repo.q.DiscoverySubcategoryModel.Status.Eq(string(entity.DiscoveryStatusActive))).
		Order(repo.q.DiscoverySubcategoryModel.CategoryID.Asc(), repo.q.DiscoverySubcategoryModel.DisplayOrder.Asc(), repo.q.DiscoverySubcategoryModel.Name.Asc()).
		Find()
	if err != nil {
		return nil, domainerrors.ErrPersistenceFailed
	}

	result := make([]*entity.DiscoverySubcategory, 0, len(subcategories))
	for _, subcategory := range subcategories {
		result = append(result, toDiscoverySubcategoryDomain(subcategory))
	}

	return result, nil
}

func (repo *discoveryRepository) ListActiveHubs(ctx context.Context) ([]*entity.Hub, error) {
	hubs, err := repo.q.HubModel.WithContext(ctx).
		Where(repo.q.HubModel.Status.Eq(string(entity.DiscoveryStatusActive))).
		Order(repo.q.HubModel.City.Asc(), repo.q.HubModel.AreaName.Asc(), repo.q.HubModel.Type.Asc(), repo.q.HubModel.Name.Asc()).
		Find()
	if err != nil {
		return nil, domainerrors.ErrPersistenceFailed
	}

	result := make([]*entity.Hub, 0, len(hubs))
	for _, hub := range hubs {
		result = append(result, toHubDomain(hub))
	}

	return result, nil
}

func discoveryCategoryLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainerrors.ErrDiscoveryCategoryNotFound
	}

	return domainerrors.ErrPersistenceFailed
}

func discoverySubcategoryLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainerrors.ErrDiscoverySubcategoryNotFound
	}

	return domainerrors.ErrPersistenceFailed
}

func hubLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainerrors.ErrHubNotFound
	}

	return domainerrors.ErrPersistenceFailed
}

func normalizeDiscoverySlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func toDiscoveryCategoryDomain(data *model.DiscoveryCategoryModel) *entity.DiscoveryCategory {
	if data == nil {
		return nil
	}

	return &entity.DiscoveryCategory{
		ID:           data.ID,
		Slug:         data.Slug,
		Name:         data.Name,
		DisplayOrder: data.DisplayOrder,
		Status:       entity.DiscoveryStatus(data.Status),
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

func toDiscoverySubcategoryDomain(data *model.DiscoverySubcategoryModel) *entity.DiscoverySubcategory {
	if data == nil {
		return nil
	}

	return &entity.DiscoverySubcategory{
		ID:           data.ID,
		CategoryID:   data.CategoryID,
		Slug:         data.Slug,
		Name:         data.Name,
		DisplayOrder: data.DisplayOrder,
		Status:       entity.DiscoveryStatus(data.Status),
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

func toHubDomain(data *model.HubModel) *entity.Hub {
	if data == nil {
		return nil
	}

	return &entity.Hub{
		ID:        data.ID,
		Slug:      data.Slug,
		Name:      data.Name,
		Type:      entity.HubType(data.Type),
		City:      data.City,
		AreaName:  data.AreaName,
		StartsAt:  data.StartsAt,
		EndsAt:    data.EndsAt,
		Status:    entity.DiscoveryStatus(data.Status),
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}
}
