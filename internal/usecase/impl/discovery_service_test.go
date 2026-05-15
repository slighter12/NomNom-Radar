package impl

import (
	"context"
	"errors"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type discoveryRepositoryStub struct {
	findCategoryByIDFunc        func(context.Context, uuid.UUID) (*entity.DiscoveryCategory, error)
	findCategoryBySlugFunc      func(context.Context, string) (*entity.DiscoveryCategory, error)
	findSubcategoryByIDFunc     func(context.Context, uuid.UUID) (*entity.DiscoverySubcategory, error)
	findSubcategoryBySlugFunc   func(context.Context, string) (*entity.DiscoverySubcategory, error)
	findHubByIDFunc             func(context.Context, uuid.UUID) (*entity.Hub, error)
	findHubBySlugFunc           func(context.Context, string) (*entity.Hub, error)
	listActiveCategoriesFunc    func(context.Context) ([]*entity.DiscoveryCategory, error)
	listActiveSubcategoriesFunc func(context.Context) ([]*entity.DiscoverySubcategory, error)
	listActiveHubsFunc          func(context.Context) ([]*entity.Hub, error)
	searchPublicMerchantsFunc   func(context.Context, *repository.PublicMerchantSearchFilter) ([]*entity.PublicMerchantSearchItem, int64, error)
}

func (s *discoveryRepositoryStub) FindCategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoveryCategory, error) {
	return s.findCategoryByIDFunc(ctx, id)
}

func (s *discoveryRepositoryStub) FindCategoryBySlug(ctx context.Context, slug string) (*entity.DiscoveryCategory, error) {
	return s.findCategoryBySlugFunc(ctx, slug)
}

func (s *discoveryRepositoryStub) FindSubcategoryByID(ctx context.Context, id uuid.UUID) (*entity.DiscoverySubcategory, error) {
	return s.findSubcategoryByIDFunc(ctx, id)
}

func (s *discoveryRepositoryStub) FindSubcategoryBySlug(ctx context.Context, slug string) (*entity.DiscoverySubcategory, error) {
	return s.findSubcategoryBySlugFunc(ctx, slug)
}

func (s *discoveryRepositoryStub) FindHubByID(ctx context.Context, id uuid.UUID) (*entity.Hub, error) {
	return s.findHubByIDFunc(ctx, id)
}

func (s *discoveryRepositoryStub) FindHubBySlug(ctx context.Context, slug string) (*entity.Hub, error) {
	return s.findHubBySlugFunc(ctx, slug)
}

func (s *discoveryRepositoryStub) ListActiveCategories(ctx context.Context) ([]*entity.DiscoveryCategory, error) {
	return s.listActiveCategoriesFunc(ctx)
}

func (s *discoveryRepositoryStub) ListActiveSubcategories(ctx context.Context) ([]*entity.DiscoverySubcategory, error) {
	return s.listActiveSubcategoriesFunc(ctx)
}

func (s *discoveryRepositoryStub) ListActiveHubs(ctx context.Context) ([]*entity.Hub, error) {
	return s.listActiveHubsFunc(ctx)
}

func (s *discoveryRepositoryStub) SearchPublicMerchants(ctx context.Context, filter *repository.PublicMerchantSearchFilter) ([]*entity.PublicMerchantSearchItem, int64, error) {
	return s.searchPublicMerchantsFunc(ctx, filter)
}

func TestDiscoveryService_ListActiveCategories_GroupsSubcategories(t *testing.T) {
	ctx := context.Background()
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	service := NewDiscoveryService(DiscoveryServiceParams{
		DiscoveryRepo: &discoveryRepositoryStub{
			listActiveCategoriesFunc: func(context.Context) ([]*entity.DiscoveryCategory, error) {
				return []*entity.DiscoveryCategory{
					{ID: categoryID, Slug: "meal", Name: "Meal", DisplayOrder: 1, Status: entity.DiscoveryStatusActive},
				}, nil
			},
			listActiveSubcategoriesFunc: func(context.Context) ([]*entity.DiscoverySubcategory, error) {
				return []*entity.DiscoverySubcategory{
					{ID: subcategoryID, CategoryID: categoryID, Slug: "grill", Name: "Grill", Status: entity.DiscoveryStatusActive},
				}, nil
			},
		},
	})

	result, err := service.ListActiveCategories(ctx)

	require.NoError(t, err)
	require.Len(t, result.Categories, 1)
	assert.Equal(t, "meal", result.Categories[0].Slug)
	require.Len(t, result.Categories[0].Subcategories, 1)
	assert.Equal(t, "grill", result.Categories[0].Subcategories[0].Slug)
}

func TestDiscoveryService_ListActiveHubs_ReturnsHubs(t *testing.T) {
	hubID := uuid.New()
	service := NewDiscoveryService(DiscoveryServiceParams{
		DiscoveryRepo: &discoveryRepositoryStub{
			listActiveHubsFunc: func(context.Context) ([]*entity.Hub, error) {
				return []*entity.Hub{{ID: hubID, Slug: "night-market", Status: entity.DiscoveryStatusActive}}, nil
			},
		},
	})

	result, err := service.ListActiveHubs(context.Background())

	require.NoError(t, err)
	require.Len(t, result.Hubs, 1)
	assert.Equal(t, hubID, result.Hubs[0].ID)
}

func TestDiscoveryService_SearchPublicMerchants_RejectsConflictingFilterRefs(t *testing.T) {
	categoryID := uuid.New()
	service := NewDiscoveryService(DiscoveryServiceParams{DiscoveryRepo: &discoveryRepositoryStub{}})

	result, err := service.SearchPublicMerchants(context.Background(), &usecase.SearchPublicMerchantsInput{
		Category: usecase.DiscoveryFilterRef{ID: &categoryID, Slug: "meal"},
		Page:     1,
		PageSize: 20,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
	assert.Contains(t, appErrorDetails(t, err), "category_id and category_slug")
}

func TestDiscoveryService_SearchPublicMerchants_RejectsInactiveDiscoveryFilters(t *testing.T) {
	categoryID := uuid.New()
	service := NewDiscoveryService(DiscoveryServiceParams{
		DiscoveryRepo: &discoveryRepositoryStub{
			findCategoryByIDFunc: func(context.Context, uuid.UUID) (*entity.DiscoveryCategory, error) {
				return &entity.DiscoveryCategory{ID: categoryID, Status: entity.DiscoveryStatusInactive}, nil
			},
		},
	})

	result, err := service.SearchPublicMerchants(context.Background(), &usecase.SearchPublicMerchantsInput{
		Category: usecase.DiscoveryFilterRef{ID: &categoryID},
		Page:     1,
		PageSize: 20,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
	assert.Contains(t, appErrorDetails(t, err), "active category")
}

func TestDiscoveryService_SearchPublicMerchants_RejectsMismatchedSubcategoryFilter(t *testing.T) {
	categoryID := uuid.New()
	otherCategoryID := uuid.New()
	subcategoryID := uuid.New()
	service := NewDiscoveryService(DiscoveryServiceParams{
		DiscoveryRepo: &discoveryRepositoryStub{
			findCategoryByIDFunc: func(context.Context, uuid.UUID) (*entity.DiscoveryCategory, error) {
				return &entity.DiscoveryCategory{ID: categoryID, Status: entity.DiscoveryStatusActive}, nil
			},
			findSubcategoryByIDFunc: func(context.Context, uuid.UUID) (*entity.DiscoverySubcategory, error) {
				return &entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: otherCategoryID, Status: entity.DiscoveryStatusActive}, nil
			},
		},
	})

	result, err := service.SearchPublicMerchants(context.Background(), &usecase.SearchPublicMerchantsInput{
		Category:    usecase.DiscoveryFilterRef{ID: &categoryID},
		Subcategory: usecase.DiscoveryFilterRef{ID: &subcategoryID},
		Page:        1,
		PageSize:    20,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
	assert.Contains(t, appErrorDetails(t, err), "subcategory filter must belong")
}

func TestDiscoveryService_SearchPublicMerchants_DefaultsCoordinateRadius(t *testing.T) {
	lat := 25.033
	lon := 121.565
	var gotFilter repository.PublicMerchantSearchFilter
	service := NewDiscoveryService(DiscoveryServiceParams{
		DiscoveryRepo: &discoveryRepositoryStub{
			searchPublicMerchantsFunc: func(_ context.Context, filter *repository.PublicMerchantSearchFilter) ([]*entity.PublicMerchantSearchItem, int64, error) {
				gotFilter = *filter

				return []*entity.PublicMerchantSearchItem{}, 0, nil
			},
		},
	})

	result, err := service.SearchPublicMerchants(context.Background(), &usecase.SearchPublicMerchantsInput{
		Latitude:  &lat,
		Longitude: &lon,
		Page:      1,
		PageSize:  20,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, defaultMerchantSearchRadiusMeters, gotFilter.RadiusMeters)
	require.NotNil(t, gotFilter.Latitude)
	assert.Equal(t, lat, *gotFilter.Latitude)
	require.NotNil(t, gotFilter.Longitude)
	assert.Equal(t, lon, *gotFilter.Longitude)
	assert.Empty(t, result.Merchants)
}

func TestDiscoveryService_SearchPublicMerchants_CapsCoordinateRadius(t *testing.T) {
	lat := 25.033
	lon := 121.565
	radius := maxMerchantSearchRadiusMeters + 5000
	var gotFilter repository.PublicMerchantSearchFilter
	service := NewDiscoveryService(DiscoveryServiceParams{
		DiscoveryRepo: &discoveryRepositoryStub{
			searchPublicMerchantsFunc: func(_ context.Context, filter *repository.PublicMerchantSearchFilter) ([]*entity.PublicMerchantSearchItem, int64, error) {
				gotFilter = *filter

				return nil, 0, nil
			},
		},
	})

	_, err := service.SearchPublicMerchants(context.Background(), &usecase.SearchPublicMerchantsInput{
		Latitude:     &lat,
		Longitude:    &lon,
		RadiusMeters: &radius,
		Page:         1,
		PageSize:     20,
	})

	require.NoError(t, err)
	assert.Equal(t, maxMerchantSearchRadiusMeters, gotFilter.RadiusMeters)
}

func TestDiscoveryService_SearchPublicMerchants_RejectsPartialCoordinates(t *testing.T) {
	lat := 25.033
	service := NewDiscoveryService(DiscoveryServiceParams{DiscoveryRepo: &discoveryRepositoryStub{}})

	result, err := service.SearchPublicMerchants(context.Background(), &usecase.SearchPublicMerchantsInput{
		Latitude: &lat,
		Page:     1,
		PageSize: 20,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
	assert.Contains(t, appErrorDetails(t, err), "latitude and longitude")
}

func appErrorDetails(t *testing.T, err error) string {
	t.Helper()

	var appErr domainerrors.AppError
	require.True(t, errors.As(err, &appErr))

	return appErr.Details()
}
