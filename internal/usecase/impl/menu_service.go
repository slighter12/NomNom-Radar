package impl

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

type menuService struct {
	menuRepo      repository.MenuRepository
	userRepo      repository.UserRepository
	discoveryRepo repository.DiscoveryRepository
}

type MenuServiceParams struct {
	fx.In

	MenuRepo      repository.MenuRepository
	UserRepo      repository.UserRepository
	DiscoveryRepo repository.DiscoveryRepository
}

func NewMenuService(params MenuServiceParams) usecase.MenuUsecase {
	return &menuService{
		menuRepo:      params.MenuRepo,
		userRepo:      params.UserRepo,
		discoveryRepo: params.DiscoveryRepo,
	}
}

func (s *menuService) GetPublicMerchantMenu(ctx context.Context, merchantID uuid.UUID, input *usecase.ListMerchantMenuItemsInput) (*usecase.MerchantMenuItemsResult, error) {
	if input == nil {
		return nil, domainerrors.ErrValidationFailed.WithDetails("menu item list input is required")
	}

	if err := s.validatePublicMerchant(ctx, merchantID); err != nil {
		return nil, err
	}

	isAvailable := true
	publicInput := *input
	publicInput.IsAvailable = &isAvailable

	return s.ListMerchantMenuItems(ctx, merchantID, &publicInput)
}

func (s *menuService) ListMerchantMenuItems(ctx context.Context, merchantID uuid.UUID, input *usecase.ListMerchantMenuItemsInput) (*usecase.MerchantMenuItemsResult, error) {
	if input == nil {
		return nil, domainerrors.ErrValidationFailed.WithDetails("menu item list input is required")
	}

	page := input.Page
	pageSize := input.PageSize

	filter := repository.MenuItemListFilter{
		IsAvailable: nil,
		Keyword:     "",
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}

	if input.IsAvailable != nil {
		filter.IsAvailable = input.IsAvailable
	}
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		filter.Keyword = keyword
	}
	if input.CategoryID != nil {
		if err := s.validateActiveMenuCategory(ctx, *input.CategoryID); err != nil {
			return nil, err
		}
		filter.CategoryID = input.CategoryID
	}

	items, total, err := s.menuRepo.ListMenuItemsByMerchant(ctx, merchantID, filter)
	if err != nil {
		return nil, err
	}

	return &usecase.MerchantMenuItemsResult{
		Items: items,
		Pagination: &usecase.MerchantMenuPagination{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (s *menuService) CreateMenuItem(ctx context.Context, merchantID uuid.UUID, input *usecase.CreateMenuItemInput) (*entity.MenuItem, error) {
	if input == nil {
		return nil, domainerrors.ErrValidationFailed.WithDetails("menu item input is required")
	}

	isAvailable := true
	if input.IsAvailable != nil {
		isAvailable = *input.IsAvailable
	}

	isPopular := false
	if input.IsPopular != nil {
		isPopular = *input.IsPopular
	}

	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	name := strings.TrimSpace(input.Name)
	description := normalizeOptionalString(input.Description)
	imageURL := normalizeOptionalString(input.ImageURL)
	externalURL := normalizeOptionalString(input.ExternalURL)

	if err := validateMenuItemFields(name, description, input.CategoryID, input.Price, currency, input.PrepMinutes); err != nil {
		return nil, err
	}
	if err := s.validateActiveMenuCategory(ctx, input.CategoryID); err != nil {
		return nil, err
	}

	categoryID := input.CategoryID
	item := &entity.MenuItem{
		ID:          uuid.New(),
		MerchantID:  merchantID,
		Name:        name,
		Description: description,
		CategoryID:  &categoryID,
		Price:       input.Price,
		Currency:    currency,
		PrepMinutes: input.PrepMinutes,
		IsAvailable: isAvailable,
		IsPopular:   isPopular,
		ImageURL:    imageURL,
		ExternalURL: externalURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.menuRepo.CreateMenuItem(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *menuService) UpdateMenuItem(ctx context.Context, merchantID, itemID uuid.UUID, input *usecase.UpdateMenuItemInput) (*entity.MenuItem, error) {
	if input == nil {
		return nil, domainerrors.ErrValidationFailed.WithDetails("menu item input is required")
	}

	item, err := s.findOwnedMenuItem(ctx, merchantID, itemID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	description := normalizeOptionalString(input.Description)
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	imageURL := normalizeOptionalString(input.ImageURL)
	externalURL := normalizeOptionalString(input.ExternalURL)

	if err := validateMenuItemFields(name, description, input.CategoryID, input.Price, currency, input.PrepMinutes); err != nil {
		return nil, err
	}
	if err := s.validateActiveMenuCategory(ctx, input.CategoryID); err != nil {
		return nil, err
	}

	categoryID := input.CategoryID
	item.Name = name
	item.Description = description
	item.CategoryID = &categoryID
	item.Price = input.Price
	item.Currency = currency
	item.PrepMinutes = input.PrepMinutes
	item.IsAvailable = input.IsAvailable
	item.IsPopular = input.IsPopular
	item.ImageURL = imageURL
	item.ExternalURL = externalURL
	item.UpdatedAt = time.Now()

	if err := s.menuRepo.UpdateMenuItem(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *menuService) UpdateMenuItemStatus(ctx context.Context, merchantID, itemID uuid.UUID, isAvailable bool) (*entity.MenuItem, error) {
	if _, err := s.findOwnedMenuItem(ctx, merchantID, itemID); err != nil {
		return nil, err
	}

	if err := s.menuRepo.UpdateMenuItemAvailability(ctx, itemID, isAvailable); err != nil {
		return nil, err
	}

	updatedItem, err := s.findOwnedMenuItem(ctx, merchantID, itemID)
	if err != nil {
		return nil, err
	}

	return updatedItem, nil
}

func (s *menuService) ReorderMenuItems(ctx context.Context, merchantID uuid.UUID, input *usecase.ReorderMenuItemsInput) (*usecase.ReorderMenuItemsResult, error) {
	if err := validateReorderMenuItemsInput(input); err != nil {
		return nil, err
	}

	currentItemIDs, err := s.menuRepo.ListActiveMenuItemIDsByMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	if err := validateCompleteMenuReorder(currentItemIDs, input.ItemIDs); err != nil {
		return nil, err
	}

	if err := s.menuRepo.ReorderMenuItems(ctx, merchantID, input.ItemIDs); err != nil {
		return nil, err
	}

	return &usecase.ReorderMenuItemsResult{UpdatedCount: len(input.ItemIDs)}, nil
}

func (s *menuService) DeleteMenuItem(ctx context.Context, merchantID, itemID uuid.UUID) error {
	item, err := s.findOwnedMenuItem(ctx, merchantID, itemID)
	if err != nil {
		return err
	}

	if err := s.menuRepo.DeleteMenuItem(ctx, merchantID, item.ID); err != nil {
		return err
	}

	return nil
}

func (s *menuService) findOwnedMenuItem(ctx context.Context, merchantID, itemID uuid.UUID) (*entity.MenuItem, error) {
	item, err := s.menuRepo.FindMenuItemByID(ctx, itemID)
	if err != nil {
		return nil, err
	}

	if item.MerchantID != merchantID {
		return nil, domainerrors.ErrForbiddenResourceOwner
	}

	return item, nil
}

func (s *menuService) validatePublicMerchant(ctx context.Context, merchantID uuid.UUID) error {
	merchant, err := s.userRepo.FindByID(ctx, merchantID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrUserNotFound) {
			return replaceWithSourceStack(err, domainerrors.ErrMerchantNotFound)
		}

		return err
	}

	if merchant.MerchantProfile == nil {
		return domainerrors.ErrMerchantNotFound
	}

	return nil
}

func (s *menuService) validateActiveMenuCategory(ctx context.Context, categoryID uuid.UUID) error {
	if categoryID == uuid.Nil {
		return domainerrors.ErrValidationFailed.WithDetails("category_id is required")
	}

	subcategory, err := s.discoveryRepo.FindSubcategoryByID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrDiscoverySubcategoryNotFound) {
			return replaceWithSourceStack(err, domainerrors.ErrValidationFailed.WithDetails("category_id must reference an active discovery subcategory"))
		}

		return err
	}
	if subcategory.Status != entity.DiscoveryStatusActive {
		return domainerrors.ErrValidationFailed.WithDetails("category_id must reference an active discovery subcategory")
	}

	return nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func validateMenuItemFields(name string, description *string, categoryID uuid.UUID, price int, currency string, prepMinutes int) error {
	if name == "" {
		return domainerrors.ErrValidationFailed.WithDetails("name is required")
	}
	if utf8.RuneCountInString(name) > 80 {
		return domainerrors.ErrValidationFailed.WithDetails("name must be 80 characters or fewer")
	}
	if description != nil && utf8.RuneCountInString(*description) > 500 {
		return domainerrors.ErrValidationFailed.WithDetails("description must be 500 characters or fewer")
	}
	if categoryID == uuid.Nil {
		return domainerrors.ErrValidationFailed.WithDetails("category_id is required")
	}
	if price < 0 {
		return domainerrors.ErrValidationFailed.WithDetails("price must be greater than or equal to zero")
	}
	if currency != entity.CurrencyTWD {
		return domainerrors.ErrValidationFailed.WithDetails("currency must be TWD")
	}
	if prepMinutes <= 0 {
		return domainerrors.ErrValidationFailed.WithDetails("prep_minutes must be greater than zero")
	}

	return nil
}

func validateReorderMenuItemsInput(input *usecase.ReorderMenuItemsInput) error {
	if input == nil || len(input.ItemIDs) == 0 {
		return domainerrors.ErrValidationFailed.WithDetails("item_ids must not be empty")
	}

	seenIDs := make(map[uuid.UUID]struct{}, len(input.ItemIDs))
	for _, itemID := range input.ItemIDs {
		if itemID == uuid.Nil {
			return domainerrors.ErrValidationFailed.WithDetails("item_ids must not contain nil uuid")
		}
		if _, exists := seenIDs[itemID]; exists {
			return domainerrors.ErrValidationFailed.WithDetails("duplicate menu item id in reorder request")
		}
		seenIDs[itemID] = struct{}{}
	}

	return nil
}

func validateCompleteMenuReorder(existingItemIDs, reorderedItemIDs []uuid.UUID) error {
	if len(existingItemIDs) != len(reorderedItemIDs) {
		return domainerrors.ErrValidationFailed.WithDetails("reorder request must include all active menu item ids")
	}

	existingSet := make(map[uuid.UUID]struct{}, len(existingItemIDs))
	for _, itemID := range existingItemIDs {
		existingSet[itemID] = struct{}{}
	}

	for _, itemID := range reorderedItemIDs {
		if _, exists := existingSet[itemID]; !exists {
			return domainerrors.ErrValidationFailed.WithDetails("reorder request contains menu item outside merchant scope")
		}
	}

	return nil
}
