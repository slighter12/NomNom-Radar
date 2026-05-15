package impl

import (
	"context"
	"strings"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

const (
	defaultMerchantSearchRadiusMeters = 3000
	maxMerchantSearchRadiusMeters     = 10000
)

type discoveryService struct {
	discoveryRepo repository.DiscoveryRepository
}

type DiscoveryServiceParams struct {
	fx.In

	DiscoveryRepo repository.DiscoveryRepository
}

func NewDiscoveryService(params DiscoveryServiceParams) usecase.DiscoveryUsecase {
	return &discoveryService{
		discoveryRepo: params.DiscoveryRepo,
	}
}

func (s *discoveryService) ListActiveCategories(
	ctx context.Context,
) (*usecase.ListDiscoveryCategoriesResult, error) {
	categories, err := s.discoveryRepo.ListActiveCategories(ctx)
	if err != nil {
		return nil, err
	}

	subcategories, err := s.discoveryRepo.ListActiveSubcategories(ctx)
	if err != nil {
		return nil, err
	}

	groupedSubcategories := make(map[uuid.UUID][]*usecase.DiscoverySubcategoryResult, len(categories))
	for _, subcategory := range subcategories {
		if subcategory == nil || subcategory.Status != entity.DiscoveryStatusActive {
			continue
		}
		groupedSubcategories[subcategory.CategoryID] = append(
			groupedSubcategories[subcategory.CategoryID],
			toDiscoverySubcategoryResult(subcategory),
		)
	}

	result := &usecase.ListDiscoveryCategoriesResult{
		Categories: make([]*usecase.DiscoveryCategoryResult, 0, len(categories)),
	}
	for _, category := range categories {
		if category == nil || category.Status != entity.DiscoveryStatusActive {
			continue
		}
		result.Categories = append(result.Categories, &usecase.DiscoveryCategoryResult{
			ID:            category.ID,
			Slug:          category.Slug,
			Name:          category.Name,
			DisplayOrder:  category.DisplayOrder,
			Subcategories: groupedSubcategories[category.ID],
		})
	}

	return result, nil
}

func (s *discoveryService) ListActiveSubcategories(
	ctx context.Context,
) (*usecase.ListDiscoverySubcategoriesResult, error) {
	subcategories, err := s.discoveryRepo.ListActiveSubcategories(ctx)
	if err != nil {
		return nil, err
	}

	result := &usecase.ListDiscoverySubcategoriesResult{
		Subcategories: make([]*usecase.DiscoverySubcategoryResult, 0, len(subcategories)),
	}
	for _, subcategory := range subcategories {
		if subcategory == nil || subcategory.Status != entity.DiscoveryStatusActive {
			continue
		}
		result.Subcategories = append(result.Subcategories, toDiscoverySubcategoryResult(subcategory))
	}

	return result, nil
}

func (s *discoveryService) ListActiveHubs(
	ctx context.Context,
) (*usecase.ListDiscoveryHubsResult, error) {
	hubs, err := s.discoveryRepo.ListActiveHubs(ctx)
	if err != nil {
		return nil, err
	}

	result := &usecase.ListDiscoveryHubsResult{
		Hubs: make([]*usecase.DiscoveryHubResult, 0, len(hubs)),
	}
	for _, hub := range hubs {
		if hub == nil || hub.Status != entity.DiscoveryStatusActive {
			continue
		}
		result.Hubs = append(result.Hubs, toDiscoveryHubResult(hub))
	}

	return result, nil
}

func (s *discoveryService) SearchPublicMerchants(
	ctx context.Context,
	input *usecase.SearchPublicMerchantsInput,
) (*usecase.SearchPublicMerchantsResult, error) {
	if input == nil {
		return nil, domainerrors.ErrValidationFailed.WithDetails("merchant search input is required")
	}

	filter, err := s.buildPublicMerchantSearchFilter(ctx, input)
	if err != nil {
		return nil, err
	}

	merchants, total, err := s.discoveryRepo.SearchPublicMerchants(ctx, &filter)
	if err != nil {
		return nil, err
	}

	return &usecase.SearchPublicMerchantsResult{
		Merchants: merchants,
		Pagination: &usecase.MerchantSearchPagination{
			Page:     input.Page,
			PageSize: input.PageSize,
			Total:    total,
		},
	}, nil
}

func (s *discoveryService) buildPublicMerchantSearchFilter(
	ctx context.Context,
	input *usecase.SearchPublicMerchantsInput,
) (repository.PublicMerchantSearchFilter, error) {
	if err := validateMerchantSearchPagination(input); err != nil {
		return repository.PublicMerchantSearchFilter{}, err
	}

	category, subcategory, hub, err := s.resolveMerchantSearchDiscoveryFilters(ctx, input)
	if err != nil {
		return repository.PublicMerchantSearchFilter{}, err
	}

	filter := repository.PublicMerchantSearchFilter{
		Keyword: strings.TrimSpace(input.Keyword),
		Limit:   input.PageSize,
		Offset:  (input.Page - 1) * input.PageSize,
	}
	if category != nil {
		filter.CategoryID = &category.ID
	}
	if subcategory != nil {
		filter.SubcategoryID = &subcategory.ID
	}
	if hub != nil {
		filter.HubID = &hub.ID
	}

	if err := applyCoordinateSearchFilter(input, &filter); err != nil {
		return repository.PublicMerchantSearchFilter{}, err
	}

	return filter, nil
}

func validateMerchantSearchPagination(input *usecase.SearchPublicMerchantsInput) error {
	if input.Page <= 0 {
		return domainerrors.ErrValidationFailed.WithDetails("page must be greater than zero")
	}
	if input.PageSize <= 0 {
		return domainerrors.ErrValidationFailed.WithDetails("page_size must be greater than zero")
	}

	return nil
}

func (s *discoveryService) resolveMerchantSearchDiscoveryFilters(
	ctx context.Context,
	input *usecase.SearchPublicMerchantsInput,
) (*entity.DiscoveryCategory, *entity.DiscoverySubcategory, *entity.Hub, error) {
	category, err := s.resolveActiveDiscoveryCategory(ctx, input.Category)
	if err != nil {
		return nil, nil, nil, err
	}
	subcategory, err := s.resolveActiveDiscoverySubcategory(ctx, input.Subcategory)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.validateCategorySubcategoryFilter(ctx, category, subcategory); err != nil {
		return nil, nil, nil, err
	}
	hub, err := s.resolveActiveHub(ctx, input.Hub)
	if err != nil {
		return nil, nil, nil, err
	}

	return category, subcategory, hub, nil
}

func (s *discoveryService) resolveActiveDiscoveryCategory(
	ctx context.Context,
	ref usecase.DiscoveryFilterRef,
) (*entity.DiscoveryCategory, error) {
	slug := normalizeDiscoveryFilterSlug(ref.Slug)
	if ref.ID != nil && slug != "" {
		return nil, domainerrors.ErrValidationFailed.WithDetails(
			"category_id and category_slug cannot both be provided",
		)
	}
	if ref.ID == nil && slug == "" {
		return nil, nil
	}

	var (
		category *entity.DiscoveryCategory
		err      error
	)
	if ref.ID != nil {
		category, err = s.discoveryRepo.FindCategoryByID(ctx, *ref.ID)
	} else {
		category, err = s.discoveryRepo.FindCategoryBySlug(ctx, slug)
	}
	if err != nil {
		return nil, err
	}
	if category.Status != entity.DiscoveryStatusActive {
		return nil, domainerrors.ErrValidationFailed.WithDetails(
			"category filter must reference an active category",
		)
	}

	return category, nil
}

func (s *discoveryService) resolveActiveDiscoverySubcategory(
	ctx context.Context,
	ref usecase.DiscoveryFilterRef,
) (*entity.DiscoverySubcategory, error) {
	slug := normalizeDiscoveryFilterSlug(ref.Slug)
	if ref.ID != nil && slug != "" {
		return nil, domainerrors.ErrValidationFailed.WithDetails(
			"subcategory_id and subcategory_slug cannot both be provided",
		)
	}
	if ref.ID == nil && slug == "" {
		return nil, nil
	}

	var (
		subcategory *entity.DiscoverySubcategory
		err         error
	)
	if ref.ID != nil {
		subcategory, err = s.discoveryRepo.FindSubcategoryByID(ctx, *ref.ID)
	} else {
		subcategory, err = s.discoveryRepo.FindSubcategoryBySlug(ctx, slug)
	}
	if err != nil {
		return nil, err
	}
	if subcategory.Status != entity.DiscoveryStatusActive {
		return nil, domainerrors.ErrValidationFailed.WithDetails(
			"subcategory filter must reference an active subcategory",
		)
	}

	return subcategory, nil
}

func (s *discoveryService) resolveActiveHub(
	ctx context.Context,
	ref usecase.DiscoveryFilterRef,
) (*entity.Hub, error) {
	slug := normalizeDiscoveryFilterSlug(ref.Slug)
	if ref.ID != nil && slug != "" {
		return nil, domainerrors.ErrValidationFailed.WithDetails(
			"hub_id and hub_slug cannot both be provided",
		)
	}
	if ref.ID == nil && slug == "" {
		return nil, nil
	}

	var (
		hub *entity.Hub
		err error
	)
	if ref.ID != nil {
		hub, err = s.discoveryRepo.FindHubByID(ctx, *ref.ID)
	} else {
		hub, err = s.discoveryRepo.FindHubBySlug(ctx, slug)
	}
	if err != nil {
		return nil, err
	}
	if hub.Status != entity.DiscoveryStatusActive {
		return nil, domainerrors.ErrValidationFailed.WithDetails(
			"hub filter must reference an active hub",
		)
	}

	return hub, nil
}

func (s *discoveryService) validateCategorySubcategoryFilter(
	ctx context.Context,
	category *entity.DiscoveryCategory,
	subcategory *entity.DiscoverySubcategory,
) error {
	if subcategory == nil {
		return nil
	}
	if category != nil {
		if subcategory.CategoryID != category.ID {
			return domainerrors.ErrValidationFailed.WithDetails(
				"subcategory filter must belong to category filter",
			)
		}

		return nil
	}

	parentCategory, err := s.discoveryRepo.FindCategoryByID(ctx, subcategory.CategoryID)
	if err != nil {
		return err
	}
	if parentCategory.Status != entity.DiscoveryStatusActive {
		return domainerrors.ErrValidationFailed.WithDetails(
			"subcategory filter must belong to an active category",
		)
	}

	return nil
}

func applyCoordinateSearchFilter(
	input *usecase.SearchPublicMerchantsInput,
	filter *repository.PublicMerchantSearchFilter,
) error {
	if !hasCoordinateSearchInput(input) {
		return nil
	}

	if err := validateCoordinateSearchInput(input); err != nil {
		return err
	}

	filter.Latitude = input.Latitude
	filter.Longitude = input.Longitude
	filter.RadiusMeters = normalizeMerchantSearchRadius(input.RadiusMeters)

	return nil
}

func hasCoordinateSearchInput(input *usecase.SearchPublicMerchantsInput) bool {
	return input.Latitude != nil || input.Longitude != nil
}

func validateCoordinateSearchInput(input *usecase.SearchPublicMerchantsInput) error {
	if input.Latitude == nil || input.Longitude == nil {
		return domainerrors.ErrValidationFailed.WithDetails(
			"latitude and longitude must be provided together",
		)
	}
	if *input.Latitude < -90 || *input.Latitude > 90 {
		return domainerrors.ErrValidationFailed.WithDetails("latitude must be between -90 and 90")
	}
	if *input.Longitude < -180 || *input.Longitude > 180 {
		return domainerrors.ErrValidationFailed.WithDetails("longitude must be between -180 and 180")
	}
	if input.RadiusMeters != nil && *input.RadiusMeters <= 0 {
		return domainerrors.ErrValidationFailed.WithDetails("radius_meters must be greater than zero")
	}

	return nil
}

func normalizeMerchantSearchRadius(radiusMeters *int) int {
	if radiusMeters == nil {
		return defaultMerchantSearchRadiusMeters
	}
	if *radiusMeters > maxMerchantSearchRadiusMeters {
		return maxMerchantSearchRadiusMeters
	}

	return *radiusMeters
}

func normalizeDiscoveryFilterSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func toDiscoverySubcategoryResult(subcategory *entity.DiscoverySubcategory) *usecase.DiscoverySubcategoryResult {
	if subcategory == nil {
		return nil
	}

	return &usecase.DiscoverySubcategoryResult{
		ID:           subcategory.ID,
		CategoryID:   subcategory.CategoryID,
		Slug:         subcategory.Slug,
		Name:         subcategory.Name,
		DisplayOrder: subcategory.DisplayOrder,
	}
}

func toDiscoveryHubResult(hub *entity.Hub) *usecase.DiscoveryHubResult {
	if hub == nil {
		return nil
	}

	return &usecase.DiscoveryHubResult{
		ID:       hub.ID,
		Slug:     hub.Slug,
		Name:     hub.Name,
		Type:     hub.Type,
		City:     hub.City,
		AreaName: hub.AreaName,
		StartsAt: hub.StartsAt,
		EndsAt:   hub.EndsAt,
	}
}
