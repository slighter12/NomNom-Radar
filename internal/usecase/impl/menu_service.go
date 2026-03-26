package impl

import (
	"context"
	"errors"
	"fmt"
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
	menuRepo repository.MenuRepository
	userRepo repository.UserRepository
}

type MenuServiceParams struct {
	fx.In

	MenuRepo repository.MenuRepository
	UserRepo repository.UserRepository
}

func NewMenuService(params MenuServiceParams) usecase.MenuUsecase {
	return &menuService{
		menuRepo: params.MenuRepo,
		userRepo: params.UserRepo,
	}
}

func (s *menuService) GetPublicMerchantMenu(ctx context.Context, merchantID uuid.UUID, input *usecase.ListMerchantMenuItemsInput) (*usecase.MerchantMenuItemsResult, error) {
	if input == nil {
		return nil, fmt.Errorf("menu item list input is required: %w", domainerrors.ErrValidationFailed)
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
		return nil, fmt.Errorf("menu item list input is required: %w", domainerrors.ErrValidationFailed)
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
	if input.Category != "" {
		category := entity.MenuCategory(strings.TrimSpace(input.Category))
		if !category.IsValid() {
			return nil, domainerrors.ErrInvalidMenuCategory
		}
		filter.Category = &category
	}

	items, total, err := s.menuRepo.ListMenuItemsByMerchant(ctx, merchantID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list menu items by merchant: %w", err)
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
		return nil, fmt.Errorf("menu item input is required: %w", domainerrors.ErrValidationFailed)
	}

	isAvailable := true
	if input.IsAvailable != nil {
		isAvailable = *input.IsAvailable
	}

	isPopular := false
	if input.IsPopular != nil {
		isPopular = *input.IsPopular
	}

	category := entity.MenuCategory(strings.TrimSpace(input.Category))
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	name := strings.TrimSpace(input.Name)
	description := normalizeOptionalString(input.Description)
	imageURL := normalizeOptionalString(input.ImageURL)
	externalURL := normalizeOptionalString(input.ExternalURL)

	if err := validateMenuItemFields(name, description, category, input.Price, currency, input.PrepMinutes); err != nil {
		return nil, err
	}

	item := &entity.MenuItem{
		ID:          uuid.New(),
		MerchantID:  merchantID,
		Name:        name,
		Description: description,
		Category:    category,
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
		return nil, fmt.Errorf("failed to create menu item: %w", err)
	}

	return item, nil
}

func (s *menuService) UpdateMenuItem(ctx context.Context, merchantID, itemID uuid.UUID, input *usecase.UpdateMenuItemInput) (*entity.MenuItem, error) {
	if input == nil {
		return nil, fmt.Errorf("menu item input is required: %w", domainerrors.ErrValidationFailed)
	}

	item, err := s.findOwnedMenuItem(ctx, merchantID, itemID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	description := normalizeOptionalString(input.Description)
	category := entity.MenuCategory(strings.TrimSpace(input.Category))
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	imageURL := normalizeOptionalString(input.ImageURL)
	externalURL := normalizeOptionalString(input.ExternalURL)

	if err := validateMenuItemFields(name, description, category, input.Price, currency, input.PrepMinutes); err != nil {
		return nil, err
	}

	item.Name = name
	item.Description = description
	item.Category = category
	item.Price = input.Price
	item.Currency = currency
	item.PrepMinutes = input.PrepMinutes
	item.IsAvailable = input.IsAvailable
	item.IsPopular = input.IsPopular
	item.ImageURL = imageURL
	item.ExternalURL = externalURL
	item.UpdatedAt = time.Now()

	if err := s.menuRepo.UpdateMenuItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update menu item: %w", err)
	}

	return item, nil
}

func (s *menuService) UpdateMenuItemStatus(ctx context.Context, merchantID, itemID uuid.UUID, isAvailable bool) (*entity.MenuItem, error) {
	if _, err := s.findOwnedMenuItem(ctx, merchantID, itemID); err != nil {
		return nil, err
	}

	if err := s.menuRepo.UpdateMenuItemAvailability(ctx, itemID, isAvailable); err != nil {
		if errors.Is(err, repository.ErrMenuItemNotFound) {
			return nil, domainerrors.ErrMenuItemNotFound
		}

		return nil, fmt.Errorf("failed to update menu item status: %w", err)
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
		return nil, fmt.Errorf("failed to list active menu item ids by merchant: %w", err)
	}

	if err := validateCompleteMenuReorder(currentItemIDs, input.ItemIDs); err != nil {
		return nil, err
	}

	if err := s.menuRepo.ReorderMenuItems(ctx, merchantID, input.ItemIDs); err != nil {
		return nil, fmt.Errorf("failed to reorder menu items: %w", err)
	}

	return &usecase.ReorderMenuItemsResult{UpdatedCount: len(input.ItemIDs)}, nil
}

func (s *menuService) DeleteMenuItem(ctx context.Context, merchantID, itemID uuid.UUID) error {
	item, err := s.findOwnedMenuItem(ctx, merchantID, itemID)
	if err != nil {
		return err
	}

	if err := s.menuRepo.DeleteMenuItem(ctx, merchantID, item.ID); err != nil {
		return fmt.Errorf("failed to delete menu item: %w", err)
	}

	return nil
}

func (s *menuService) findOwnedMenuItem(ctx context.Context, merchantID, itemID uuid.UUID) (*entity.MenuItem, error) {
	item, err := s.menuRepo.FindMenuItemByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, repository.ErrMenuItemNotFound) {
			return nil, domainerrors.ErrMenuItemNotFound
		}

		return nil, fmt.Errorf("failed to find menu item by id: %w", err)
	}

	if item.MerchantID != merchantID {
		return nil, domainerrors.ErrForbiddenResourceOwner
	}

	return item, nil
}

func (s *menuService) validatePublicMerchant(ctx context.Context, merchantID uuid.UUID) error {
	merchant, err := s.userRepo.FindByID(ctx, merchantID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return domainerrors.ErrMerchantNotFound
		}

		return fmt.Errorf("failed to find merchant by id: %w", err)
	}

	if merchant.MerchantProfile == nil {
		return domainerrors.ErrMerchantNotFound
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

func validateMenuItemFields(name string, description *string, category entity.MenuCategory, price int, currency string, prepMinutes int) error {
	if name == "" {
		return fmt.Errorf("name is required: %w", domainerrors.ErrValidationFailed)
	}
	if utf8.RuneCountInString(name) > 80 {
		return fmt.Errorf("name must be 80 characters or fewer: %w", domainerrors.ErrValidationFailed)
	}
	if description != nil && utf8.RuneCountInString(*description) > 500 {
		return fmt.Errorf("description must be 500 characters or fewer: %w", domainerrors.ErrValidationFailed)
	}
	if !category.IsValid() {
		return domainerrors.ErrInvalidMenuCategory
	}
	if price < 0 {
		return fmt.Errorf("price must be greater than or equal to zero: %w", domainerrors.ErrValidationFailed)
	}
	if currency != entity.CurrencyTWD {
		return fmt.Errorf("currency must be TWD: %w", domainerrors.ErrValidationFailed)
	}
	if prepMinutes <= 0 {
		return fmt.Errorf("prep_minutes must be greater than zero: %w", domainerrors.ErrValidationFailed)
	}

	return nil
}

func validateReorderMenuItemsInput(input *usecase.ReorderMenuItemsInput) error {
	if input == nil || len(input.ItemIDs) == 0 {
		return fmt.Errorf("item_ids must not be empty: %w", domainerrors.ErrValidationFailed)
	}

	seenIDs := make(map[uuid.UUID]struct{}, len(input.ItemIDs))
	for _, itemID := range input.ItemIDs {
		if itemID == uuid.Nil {
			return fmt.Errorf("item_ids must not contain nil uuid: %w", domainerrors.ErrValidationFailed)
		}
		if _, exists := seenIDs[itemID]; exists {
			return fmt.Errorf("duplicate menu item id in reorder request: %w", domainerrors.ErrValidationFailed)
		}
		seenIDs[itemID] = struct{}{}
	}

	return nil
}

func validateCompleteMenuReorder(existingItemIDs, reorderedItemIDs []uuid.UUID) error {
	if len(existingItemIDs) != len(reorderedItemIDs) {
		return fmt.Errorf("reorder request must include all active menu item ids: %w", domainerrors.ErrValidationFailed)
	}

	existingSet := make(map[uuid.UUID]struct{}, len(existingItemIDs))
	for _, itemID := range existingItemIDs {
		existingSet[itemID] = struct{}{}
	}

	for _, itemID := range reorderedItemIDs {
		if _, exists := existingSet[itemID]; !exists {
			return fmt.Errorf("reorder request contains menu item outside merchant scope: %w", domainerrors.ErrValidationFailed)
		}
	}

	return nil
}
