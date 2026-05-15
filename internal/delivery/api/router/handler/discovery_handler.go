package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"radar/internal/delivery/api/response"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type DiscoveryHandlerParams struct {
	fx.In

	DiscoveryUC usecase.DiscoveryUsecase
	Logger      *slog.Logger
}

type DiscoveryHandler struct {
	discoveryUC usecase.DiscoveryUsecase
	logger      *slog.Logger
}

type SearchPublicMerchantsQueryParams struct {
	PaginationQueryParams
	Keyword         string   `query:"keyword"`
	CategoryID      string   `query:"category_id" validate:"omitempty,uuid"`
	CategorySlug    string   `query:"category_slug"`
	SubcategoryID   string   `query:"subcategory_id" validate:"omitempty,uuid"`
	SubcategorySlug string   `query:"subcategory_slug"`
	HubID           string   `query:"hub_id" validate:"omitempty,uuid"`
	HubSlug         string   `query:"hub_slug"`
	Latitude        *float64 `query:"latitude" validate:"omitempty,min=-90,max=90"`
	Longitude       *float64 `query:"longitude" validate:"omitempty,min=-180,max=180"`
	RadiusMeters    *int     `query:"radius_meters" validate:"omitempty,gte=1"`
}

const (
	defaultMerchantSearchPage     = 1
	defaultMerchantSearchPageSize = 20
	maxMerchantSearchPageSize     = 100
)

func NewDiscoveryHandler(params DiscoveryHandlerParams) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryUC: params.DiscoveryUC,
		logger:      params.Logger,
	}
}

func (h *DiscoveryHandler) ListActiveCategories(c echo.Context) error {
	result, err := h.discoveryUC.ListActiveCategories(c.Request().Context())
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *DiscoveryHandler) ListActiveSubcategories(c echo.Context) error {
	result, err := h.discoveryUC.ListActiveSubcategories(c.Request().Context())
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *DiscoveryHandler) ListActiveHubs(c echo.Context) error {
	result, err := h.discoveryUC.ListActiveHubs(c.Request().Context())
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *DiscoveryHandler) SearchPublicMerchants(c echo.Context) error {
	input, err := h.parseSearchPublicMerchantsInput(c)
	if err != nil {
		return err
	}

	result, err := h.discoveryUC.SearchPublicMerchants(c.Request().Context(), input)
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *DiscoveryHandler) parseSearchPublicMerchantsInput(
	c echo.Context,
) (*usecase.SearchPublicMerchantsInput, error) {
	query := newSearchPublicMerchantsQueryParams()
	if err := bindQueryParams(c, &query); err != nil {
		return nil, handleSearchPublicMerchantsQueryBindingError(c)
	}

	if err := validateRequest(c, &query); err != nil {
		return nil, err
	}

	query.PageSize = min(query.PageSize, maxMerchantSearchPageSize)

	categoryID, err := parseOptionalUUIDQueryValue(c, "category_id", query.CategoryID)
	if err != nil {
		return nil, err
	}
	subcategoryID, err := parseOptionalUUIDQueryValue(c, "subcategory_id", query.SubcategoryID)
	if err != nil {
		return nil, err
	}
	hubID, err := parseOptionalUUIDQueryValue(c, "hub_id", query.HubID)
	if err != nil {
		return nil, err
	}

	return &usecase.SearchPublicMerchantsInput{
		Keyword: strings.TrimSpace(query.Keyword),
		Category: usecase.DiscoveryFilterRef{
			ID:   categoryID,
			Slug: strings.TrimSpace(query.CategorySlug),
		},
		Subcategory: usecase.DiscoveryFilterRef{
			ID:   subcategoryID,
			Slug: strings.TrimSpace(query.SubcategorySlug),
		},
		Hub: usecase.DiscoveryFilterRef{
			ID:   hubID,
			Slug: strings.TrimSpace(query.HubSlug),
		},
		Latitude:     query.Latitude,
		Longitude:    query.Longitude,
		RadiusMeters: query.RadiusMeters,
		Page:         query.Page,
		PageSize:     query.PageSize,
	}, nil
}

func newSearchPublicMerchantsQueryParams() SearchPublicMerchantsQueryParams {
	return SearchPublicMerchantsQueryParams{
		PaginationQueryParams: NewPaginationQueryParams(defaultMerchantSearchPage, defaultMerchantSearchPageSize),
	}
}

func parseOptionalUUIDQueryValue(c echo.Context, name string, rawValue string) (*uuid.UUID, error) {
	rawValue = strings.TrimSpace(rawValue)
	if rawValue == "" {
		return nil, nil
	}

	value, err := uuid.Parse(rawValue)
	if err != nil {
		return nil, abortHandledResponse(response.BadRequest(
			c,
			"VALIDATION_ERROR",
			name+" 必須為有效的 UUID",
		))
	}

	return &value, nil
}

func handleSearchPublicMerchantsQueryBindingError(c echo.Context) error {
	for _, name := range []string{"page", "page_size", "radius_meters"} {
		rawValue := strings.TrimSpace(c.QueryParam(name))
		if rawValue == "" {
			continue
		}
		if _, err := strconv.Atoi(rawValue); err != nil {
			return abortHandledResponse(response.BadRequest(c, "VALIDATION_ERROR", name+" 必須為整數"))
		}
	}

	for _, name := range []string{"latitude", "longitude"} {
		rawValue := strings.TrimSpace(c.QueryParam(name))
		if rawValue == "" {
			continue
		}
		if _, err := strconv.ParseFloat(rawValue, 64); err != nil {
			return abortHandledResponse(response.BadRequest(c, "VALIDATION_ERROR", name+" 必須為數字"))
		}
	}

	return abortHandledResponse(response.BindingError(c, "INVALID_INPUT", "Invalid merchant search query input"))
}
