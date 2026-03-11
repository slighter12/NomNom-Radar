package postgres

import (
	"context"
	"strings"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/errors"
	"radar/internal/infra/persistence/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type menuRepository struct {
	db *gorm.DB
}

const reorderMenuItemsValidationMessage = "reorder request must include all active menu item ids"

type menuItemMerchantRecord struct {
	MerchantID uuid.UUID `gorm:"column:merchant_id"`
}

func NewMenuRepository(db *gorm.DB) repository.MenuRepository {
	return &menuRepository{db: db}
}

func (repo *menuRepository) CreateMenuItem(ctx context.Context, item *entity.MenuItem) error {
	itemM := fromMenuItemDomain(item)

	return repo.withTransaction(ctx, func(tx *gorm.DB) error {
		if err := repo.lockMerchantProfileForMenuWrite(tx, item.MerchantID); err != nil {
			return err
		}

		nextDisplayOrder, err := repo.getNextDisplayOrder(tx, item.MerchantID)
		if err != nil {
			return domainerrors.NewDatabaseExecuteError(err, "failed to allocate menu item display order")
		}

		itemM.DisplayOrder = nextDisplayOrder
		item.DisplayOrder = nextDisplayOrder

		if err := tx.Create(itemM).Error; err != nil {
			return repo.toMenuItemCreateError(err)
		}

		item.ID = itemM.ID
		item.CreatedAt = itemM.CreatedAt
		item.UpdatedAt = itemM.UpdatedAt

		return nil
	})
}

func (repo *menuRepository) FindMenuItemByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	var itemM model.MenuItemModel
	err := repo.db.WithContext(ctx).
		Where("id = ?", id).
		First(&itemM).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrMenuItemNotFound
		}

		return nil, errors.WithStack(err)
	}

	return toMenuItemDomain(&itemM), nil
}

func (repo *menuRepository) ListActiveMenuItemIDsByMerchant(ctx context.Context, merchantID uuid.UUID) ([]uuid.UUID, error) {
	var itemIDs []uuid.UUID
	if err := repo.db.WithContext(ctx).
		Model(&model.MenuItemModel{}).
		Where("merchant_id = ?", merchantID).
		Order("display_order ASC").
		Pluck("id", &itemIDs).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	return itemIDs, nil
}

func (repo *menuRepository) ListMenuItemsByMerchant(ctx context.Context, merchantID uuid.UUID, filter repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
	countQuery := repo.buildMenuItemListQuery(ctx, merchantID, filter)

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}

	dataQuery := repo.buildMenuItemListQuery(ctx, merchantID, filter).
		Order("display_order ASC").
		Order("created_at ASC")
	if filter.Limit > 0 {
		dataQuery = dataQuery.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		dataQuery = dataQuery.Offset(filter.Offset)
	}

	var itemModels []*model.MenuItemModel
	if err := dataQuery.Find(&itemModels).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}

	items := make([]*entity.MenuItem, 0, len(itemModels))
	for _, itemM := range itemModels {
		items = append(items, toMenuItemDomain(itemM))
	}

	return items, total, nil
}

func (repo *menuRepository) UpdateMenuItem(ctx context.Context, item *entity.MenuItem) error {
	updates := map[string]any{
		"name":         item.Name,
		"description":  item.Description,
		"category":     item.Category,
		"price":        item.Price,
		"currency":     item.Currency,
		"prep_minutes": item.PrepMinutes,
		"is_available": item.IsAvailable,
		"is_popular":   item.IsPopular,
		"image_url":    item.ImageURL,
		"external_url": item.ExternalURL,
	}

	result := repo.db.WithContext(ctx).
		Model(&model.MenuItemModel{}).
		Where("id = ?", item.ID).
		Updates(updates)
	if result.Error != nil {
		return repo.toMenuItemUpdateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrMenuItemNotFound
	}

	updatedItem, err := repo.FindMenuItemByID(ctx, item.ID)
	if err != nil {
		return err
	}
	item.CreatedAt = updatedItem.CreatedAt
	item.UpdatedAt = updatedItem.UpdatedAt

	return nil
}

func (repo *menuRepository) UpdateMenuItemAvailability(ctx context.Context, id uuid.UUID, isAvailable bool) error {
	result := repo.db.WithContext(ctx).
		Model(&model.MenuItemModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"is_available": isAvailable,
		})
	if result.Error != nil {
		return repo.toMenuItemUpdateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrMenuItemNotFound
	}

	return nil
}

func (repo *menuRepository) DeleteMenuItem(ctx context.Context, merchantID, menuItemID uuid.UUID) error {
	return repo.withTransaction(ctx, func(tx *gorm.DB) error {
		if err := repo.lockMerchantProfileForMenuWrite(tx, merchantID); err != nil {
			return err
		}

		var itemM model.MenuItemModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND merchant_id = ?", menuItemID, merchantID).
			Take(&itemM).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrMenuItemNotFound
			}

			return errors.WithStack(err)
		}

		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", menuItemID).
			Delete(&model.MenuItemModel{}).Error; err != nil {
			return errors.WithStack(err)
		}

		if err := tx.Model(&model.MenuItemModel{}).
			Where("merchant_id = ? AND display_order > ?", merchantID, itemM.DisplayOrder).
			Update("display_order", gorm.Expr("display_order - 1")).Error; err != nil {
			return repo.toMenuItemUpdateError(err)
		}

		return nil
	})
}

func (repo *menuRepository) getNextDisplayOrder(db *gorm.DB, merchantID uuid.UUID) (int, error) {
	type nextOrderResult struct {
		NextDisplayOrder int `gorm:"column:next_display_order"`
	}

	var result nextOrderResult
	err := db.
		Model(&model.MenuItemModel{}).
		Select("COALESCE(MAX(display_order), 0) + 1 AS next_display_order").
		Where("merchant_id = ?", merchantID).
		Scan(&result).Error
	if err != nil {
		return 0, errors.WithStack(err)
	}
	if result.NextDisplayOrder <= 0 {
		return 1, nil
	}

	return result.NextDisplayOrder, nil
}

func (repo *menuRepository) lockMerchantProfileForMenuWrite(tx *gorm.DB, merchantID uuid.UUID) error {
	var lockedMerchant struct {
		UserID uuid.UUID `gorm:"column:user_id"`
	}

	err := tx.Table("merchant_profiles").
		Select("user_id").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", merchantID).
		Take(&lockedMerchant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerrors.ErrMenuItemCreationFailed.WrapMessage("invalid merchant reference")
		}

		return errors.WithStack(err)
	}

	return nil
}

func (repo *menuRepository) toMenuItemCreateError(err error) error {
	if isUniqueConstraintViolation(err) {
		return domainerrors.ErrMenuItemOrderConflict
	}
	if isForeignKeyConstraintViolation(err) {
		return domainerrors.ErrMenuItemCreationFailed.WrapMessage("invalid merchant reference")
	}
	if isNotNullConstraintViolation(err) {
		return domainerrors.ErrMenuItemCreationFailed.WrapMessage("missing required menu item information")
	}

	return domainerrors.NewDatabaseExecuteError(err, "failed to create menu item")
}

func (repo *menuRepository) toMenuItemUpdateError(err error) error {
	if isUniqueConstraintViolation(err) {
		return domainerrors.ErrMenuItemOrderConflict
	}
	if isForeignKeyConstraintViolation(err) {
		return domainerrors.ErrMenuItemUpdateFailed.WrapMessage("invalid merchant reference")
	}
	if isNotNullConstraintViolation(err) {
		return domainerrors.ErrMenuItemUpdateFailed.WrapMessage("missing required menu item information")
	}

	return domainerrors.NewDatabaseExecuteError(err, "failed to update menu item")
}

func (repo *menuRepository) ReorderMenuItems(ctx context.Context, merchantID uuid.UUID, itemIDs []uuid.UUID) error {
	if len(itemIDs) == 0 {
		return nil
	}

	return repo.withTransaction(ctx, func(tx *gorm.DB) error {
		return repo.reorderMenuItemsTx(tx, merchantID, itemIDs)
	})
}

func (repo *menuRepository) withTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if err := repo.db.WithContext(ctx).Transaction(fn); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (repo *menuRepository) reorderMenuItemsTx(tx *gorm.DB, merchantID uuid.UUID, itemIDs []uuid.UUID) error {
	if err := repo.lockMerchantProfileForMenuWrite(tx, merchantID); err != nil {
		return err
	}

	scopedItemIDs, err := repo.listScopedMenuItemIDs(tx, merchantID)
	if err != nil {
		return err
	}

	if err := repo.validateReorderMenuItems(tx, merchantID, scopedItemIDs, itemIDs); err != nil {
		return err
	}

	if err := repo.bumpMenuItemDisplayOrders(tx, merchantID, len(itemIDs)); err != nil {
		return err
	}

	return repo.applyMenuItemDisplayOrder(tx, itemIDs)
}

func (repo *menuRepository) listScopedMenuItemIDs(tx *gorm.DB, merchantID uuid.UUID) ([]uuid.UUID, error) {
	var scopedItemIDs []uuid.UUID
	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&model.MenuItemModel{}).
		Where("merchant_id = ?", merchantID).
		Order("display_order ASC").
		Pluck("id", &scopedItemIDs).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	return scopedItemIDs, nil
}

func (repo *menuRepository) validateReorderMenuItems(tx *gorm.DB, merchantID uuid.UUID, scopedItemIDs, itemIDs []uuid.UUID) error {
	providedItems, err := repo.listProvidedMenuItems(tx, itemIDs)
	if err != nil {
		return err
	}

	if len(providedItems) != len(itemIDs) {
		return domainerrors.ErrMenuItemNotFound
	}

	for idx := range providedItems {
		if providedItems[idx].MerchantID != merchantID {
			return domainerrors.ErrForbiddenResourceOwner
		}
	}

	if len(scopedItemIDs) != len(itemIDs) {
		return domainerrors.ErrValidationFailed.WrapMessage(reorderMenuItemsValidationMessage)
	}

	scopedItemSet := make(map[uuid.UUID]struct{}, len(scopedItemIDs))
	for idx := range scopedItemIDs {
		scopedItemSet[scopedItemIDs[idx]] = struct{}{}
	}

	for idx := range itemIDs {
		if _, exists := scopedItemSet[itemIDs[idx]]; !exists {
			return domainerrors.ErrValidationFailed.WrapMessage(reorderMenuItemsValidationMessage)
		}
	}

	return nil
}

func (repo *menuRepository) listProvidedMenuItems(tx *gorm.DB, itemIDs []uuid.UUID) ([]menuItemMerchantRecord, error) {
	var providedItems []menuItemMerchantRecord
	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&model.MenuItemModel{}).
		Select("merchant_id").
		Where("id IN ?", itemIDs).
		Find(&providedItems).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	return providedItems, nil
}

func (repo *menuRepository) bumpMenuItemDisplayOrders(tx *gorm.DB, merchantID uuid.UUID, itemCount int) error {
	if err := tx.Model(&model.MenuItemModel{}).
		Where("merchant_id = ?", merchantID).
		Update("display_order", gorm.Expr("display_order + ?", itemCount)).Error; err != nil {
		return repo.toMenuItemUpdateError(err)
	}

	return nil
}

func (repo *menuRepository) applyMenuItemDisplayOrder(tx *gorm.DB, itemIDs []uuid.UUID) error {
	for index, itemID := range itemIDs {
		if err := tx.Model(&model.MenuItemModel{}).
			Where("id = ?", itemID).
			Update("display_order", index+1).Error; err != nil {
			return repo.toMenuItemUpdateError(err)
		}
	}

	return nil
}

func (repo *menuRepository) buildMenuItemListQuery(ctx context.Context, merchantID uuid.UUID, filter repository.MenuItemListFilter) *gorm.DB {
	query := repo.db.WithContext(ctx).
		Model(&model.MenuItemModel{}).
		Where("merchant_id = ?", merchantID)

	if filter.Category != nil {
		query = query.Where("category = ?", *filter.Category)
	}
	if filter.IsAvailable != nil {
		query = query.Where("is_available = ?", *filter.IsAvailable)
	}
	if filter.Keyword != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("(name ILIKE ? OR COALESCE(description, '') ILIKE ?)", keyword, keyword)
	}

	return query
}

func toMenuItemDomain(data *model.MenuItemModel) *entity.MenuItem {
	if data == nil {
		return nil
	}

	return &entity.MenuItem{
		ID:           data.ID,
		MerchantID:   data.MerchantID,
		Name:         data.Name,
		Description:  data.Description,
		Category:     entity.MenuCategory(data.Category),
		Price:        data.Price,
		Currency:     data.Currency,
		PrepMinutes:  data.PrepMinutes,
		IsAvailable:  data.IsAvailable,
		IsPopular:    data.IsPopular,
		DisplayOrder: data.DisplayOrder,
		ImageURL:     data.ImageURL,
		ExternalURL:  data.ExternalURL,
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

func fromMenuItemDomain(data *entity.MenuItem) *model.MenuItemModel {
	if data == nil {
		return nil
	}

	return &model.MenuItemModel{
		ID:           data.ID,
		MerchantID:   data.MerchantID,
		Name:         data.Name,
		Description:  data.Description,
		Category:     data.Category.String(),
		Price:        data.Price,
		Currency:     data.Currency,
		PrepMinutes:  data.PrepMinutes,
		IsAvailable:  data.IsAvailable,
		IsPopular:    data.IsPopular,
		DisplayOrder: data.DisplayOrder,
		ImageURL:     data.ImageURL,
		ExternalURL:  data.ExternalURL,
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}
