package handler

import (
	"context"
	"net/http"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingDiscoveryUsecase struct {
	searchInput *usecase.SearchPublicMerchantsInput
}

type fixedDiscoveryUsecase struct {
	recordingDiscoveryUsecase
	searchResult *usecase.SearchPublicMerchantsResult
}

func (uc *recordingDiscoveryUsecase) ListActiveCategories(context.Context) (*usecase.ListDiscoveryCategoriesResult, error) {
	return &usecase.ListDiscoveryCategoriesResult{
		Categories: []*usecase.DiscoveryCategoryResult{{ID: uuid.New(), Slug: "meal", Name: "Meal"}},
	}, nil
}

func (uc *recordingDiscoveryUsecase) ListActiveSubcategories(context.Context) (*usecase.ListDiscoverySubcategoriesResult, error) {
	return &usecase.ListDiscoverySubcategoriesResult{
		Subcategories: []*usecase.DiscoverySubcategoryResult{{ID: uuid.New(), Slug: "grill"}},
	}, nil
}

func (uc *recordingDiscoveryUsecase) ListActiveHubs(context.Context) (*usecase.ListDiscoveryHubsResult, error) {
	return &usecase.ListDiscoveryHubsResult{
		Hubs: []*usecase.DiscoveryHubResult{{ID: uuid.New(), Slug: "night-market"}},
	}, nil
}

func (uc *recordingDiscoveryUsecase) SearchPublicMerchants(_ context.Context, input *usecase.SearchPublicMerchantsInput) (*usecase.SearchPublicMerchantsResult, error) {
	uc.searchInput = input

	return &usecase.SearchPublicMerchantsResult{
		Merchants: []*entity.PublicMerchantSearchItem{},
		Pagination: &usecase.MerchantSearchPagination{
			Page:     input.Page,
			PageSize: input.PageSize,
			Total:    0,
		},
	}, nil
}

func (uc *fixedDiscoveryUsecase) SearchPublicMerchants(_ context.Context, input *usecase.SearchPublicMerchantsInput) (*usecase.SearchPublicMerchantsResult, error) {
	uc.searchInput = input

	return uc.searchResult, nil
}

func TestDiscoveryHandler_ListActiveDiscoveryValues(t *testing.T) {
	handler := &DiscoveryHandler{discoveryUC: &recordingDiscoveryUsecase{}}

	tests := []struct {
		name   string
		target string
		handle func(c echo.Context) error
		body   string
	}{
		{
			name:   "categories",
			target: "/api/v1/discovery/categories",
			handle: handler.ListActiveCategories,
			body:   `"categories"`,
		},
		{
			name:   "subcategories",
			target: "/api/v1/discovery/subcategories",
			handle: handler.ListActiveSubcategories,
			body:   `"subcategories"`,
		},
		{
			name:   "hubs",
			target: "/api/v1/discovery/hubs",
			handle: handler.ListActiveHubs,
			body:   `"hubs"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newJSONContext(http.MethodGet, tt.target, "")

			err := tt.handle(c)

			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.body)
			assert.NotContains(t, rec.Body.String(), "created_at")
			assert.NotContains(t, rec.Body.String(), "updated_at")
			assert.NotContains(t, rec.Body.String(), `"status"`)
		})
	}
}

func TestDiscoveryHandler_SearchPublicMerchants_ResponseUsesPublicSummaries(t *testing.T) {
	distance := 123.4
	handler := &DiscoveryHandler{discoveryUC: &fixedDiscoveryUsecase{
		searchResult: &usecase.SearchPublicMerchantsResult{
			Merchants: []*entity.PublicMerchantSearchItem{
				{
					MerchantID:       uuid.New(),
					StoreName:        "Coffee Cart",
					StoreDescription: "Pour-over coffee",
					DiscoveryCategory: &entity.PublicDiscoveryCategorySummary{
						ID:           uuid.New(),
						Slug:         "beverage",
						Name:         "Beverage",
						DisplayOrder: 3,
					},
					DiscoverySubcategory: &entity.PublicDiscoverySubcategorySummary{
						ID:           uuid.New(),
						CategoryID:   uuid.New(),
						Slug:         "coffee",
						Name:         "Coffee",
						DisplayOrder: 1,
					},
					PrimaryLocation: &entity.PublicMerchantLocationSummary{
						ID:          uuid.New(),
						Label:       "Main spot",
						FullAddress: "Taipei 101",
						Latitude:    25.033,
						Longitude:   121.565,
					},
					DistanceMeters: &distance,
				},
			},
			Pagination: &usecase.MerchantSearchPagination{Page: 1, PageSize: 20, Total: 1},
		},
	}}
	c, rec := newJSONContext(http.MethodGet, "/api/v1/merchants?latitude=25.033&longitude=121.565", "")

	err := handler.SearchPublicMerchants(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"distance_meters":123.4`)
	assert.NotContains(t, body, "created_at")
	assert.NotContains(t, body, "updated_at")
	assert.NotContains(t, body, "owner_id")
	assert.NotContains(t, body, "owner_type")
	assert.NotContains(t, body, "is_primary")
	assert.NotContains(t, body, "is_active")
}

func TestDiscoveryHandler_SearchPublicMerchants_ParsesFilters(t *testing.T) {
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	hubID := uuid.New()
	handlerUC := &recordingDiscoveryUsecase{}
	handler := &DiscoveryHandler{discoveryUC: handlerUC}
	target := "/api/v1/merchants?keyword=coffee&category_id=" + categoryID.String() +
		"&subcategory_id=" + subcategoryID.String() +
		"&hub_id=" + hubID.String() +
		"&latitude=25.033&longitude=121.565&radius_meters=5000&page=2&page_size=200"
	c, rec := newJSONContext(http.MethodGet, target, "")

	err := handler.SearchPublicMerchants(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, handlerUC.searchInput)
	assert.Equal(t, "coffee", handlerUC.searchInput.Keyword)
	require.NotNil(t, handlerUC.searchInput.Category.ID)
	assert.Equal(t, categoryID, *handlerUC.searchInput.Category.ID)
	require.NotNil(t, handlerUC.searchInput.Subcategory.ID)
	assert.Equal(t, subcategoryID, *handlerUC.searchInput.Subcategory.ID)
	require.NotNil(t, handlerUC.searchInput.Hub.ID)
	assert.Equal(t, hubID, *handlerUC.searchInput.Hub.ID)
	require.NotNil(t, handlerUC.searchInput.Latitude)
	assert.Equal(t, 25.033, *handlerUC.searchInput.Latitude)
	require.NotNil(t, handlerUC.searchInput.Longitude)
	assert.Equal(t, 121.565, *handlerUC.searchInput.Longitude)
	require.NotNil(t, handlerUC.searchInput.RadiusMeters)
	assert.Equal(t, 5000, *handlerUC.searchInput.RadiusMeters)
	assert.Equal(t, 2, handlerUC.searchInput.Page)
	assert.Equal(t, maxMerchantSearchPageSize, handlerUC.searchInput.PageSize)
}

func TestDiscoveryHandler_SearchPublicMerchants_InvalidQueryRejected(t *testing.T) {
	handler := &DiscoveryHandler{discoveryUC: &recordingDiscoveryUsecase{}}

	tests := []struct {
		name     string
		target   string
		wantCode string
	}{
		{
			name:     "invalid uuid",
			target:   "/api/v1/merchants?category_id=not-a-uuid",
			wantCode: "VALIDATION_FAILED",
		},
		{
			name:     "invalid coordinate",
			target:   "/api/v1/merchants?latitude=abc",
			wantCode: "INVALID_INPUT",
		},
		{
			name:     "invalid page",
			target:   "/api/v1/merchants?page=0",
			wantCode: "VALIDATION_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newJSONContext(http.MethodGet, tt.target, "")

			err := handler.SearchPublicMerchants(c)
			writeTestErrorResponse(c, err)

			require.Error(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"`+tt.wantCode+`"`)
		})
	}
}
