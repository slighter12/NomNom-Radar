package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type menuRepository struct {
	q *query.Query
}

const reorderMenuItemsValidationMessage = "reorder request must include all active menu item ids"

type menuItemMerchantRecord struct {
	MerchantID uuid.UUID `gorm:"column:merchant_id"`
}

func NewMenuRepository(db *gorm.DB) repository.MenuRepository {
	return &menuRepository{q: query.Use(db)}
}

func (repo *menuRepository) CreateMenuItem(ctx context.Context, item *entity.MenuItem) error {
	itemM := fromMenuItemDomain(item)

	return repo.withTransaction(func(transactionQuery *query.Query) error {
		if err := repo.lockMerchantProfileForMenuWrite(ctx, transactionQuery, item.MerchantID); err != nil {
			return err //nolint:wrapcheck // already classified upstream when needed
		}

		nextDisplayOrder, err := repo.getNextDisplayOrder(ctx, transactionQuery, item.MerchantID)
		if err != nil {
			return err //nolint:wrapcheck // already classified upstream when needed
		}

		itemM.DisplayOrder = nextDisplayOrder
		item.DisplayOrder = nextDisplayOrder

		if err := transactionQuery.MenuItemModel.WithContext(ctx).Create(itemM); err != nil {
			return repo.toMenuItemCreateError(err)
		}

		item.ID = itemM.ID
		item.CreatedAt = itemM.CreatedAt
		item.UpdatedAt = itemM.UpdatedAt

		return nil
	})
}

func (repo *menuRepository) FindMenuItemByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	itemM, err := repo.q.MenuItemModel.WithContext(ctx).
		Where(repo.q.MenuItemModel.ID.Eq(id)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, replaceWithSourceStack(err, domainerrors.ErrMenuItemNotFound)
		}

		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	return toMenuItemDomain(itemM), nil
}

func (repo *menuRepository) ListActiveMenuItemIDsByMerchant(ctx context.Context, merchantID uuid.UUID) ([]uuid.UUID, error) {
	itemModels, err := repo.q.MenuItemModel.WithContext(ctx).
		Select(repo.q.MenuItemModel.ID).
		Where(repo.q.MenuItemModel.MerchantID.Eq(merchantID)).
		Order(repo.q.MenuItemModel.DisplayOrder.Asc()).
		Find()
	if err != nil {
		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	itemIDs := make([]uuid.UUID, 0, len(itemModels))
	for idx := range itemModels {
		itemIDs = append(itemIDs, itemModels[idx].ID)
	}

	return itemIDs, nil
}

func (repo *menuRepository) ListMenuItemsByMerchant(ctx context.Context, merchantID uuid.UUID, filter repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
	menuItem := repo.q.MenuItemModel
	conditions := []gen.Condition{menuItem.MerchantID.Eq(merchantID)}

	if filter.CategoryID != nil {
		conditions = append(conditions, menuItem.CategoryID.Eq(*filter.CategoryID))
	}
	if filter.IsAvailable != nil {
		conditions = append(conditions, menuItem.IsAvailable.Is(*filter.IsAvailable))
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		normalizedKeyword := "%" + strings.ToLower(keyword) + "%"
		conditions = append(conditions, field.Or(
			menuItem.Name.Lower().Like(normalizedKeyword),
			menuItem.Description.Lower().Like(normalizedKeyword),
		))
	}

	total, err := menuItem.WithContext(ctx).Where(conditions...).Count()
	if err != nil {
		return nil, 0, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	dataQuery := menuItem.WithContext(ctx).
		Where(conditions...).
		Order(menuItem.DisplayOrder.Asc(), menuItem.CreatedAt.Asc())
	if filter.Limit > 0 {
		dataQuery = dataQuery.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		dataQuery = dataQuery.Offset(filter.Offset)
	}

	itemModels, err := dataQuery.Find()
	if err != nil {
		return nil, 0, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	items := make([]*entity.MenuItem, 0, len(itemModels))
	for _, itemM := range itemModels {
		items = append(items, toMenuItemDomain(itemM))
	}

	return items, total, nil
}

func (repo *menuRepository) UpdateMenuItem(ctx context.Context, item *entity.MenuItem) error {
	menuItem := repo.q.MenuItemModel
	menuItemQuery := menuItem.WithContext(ctx)
	assignments := []field.AssignExpr{
		menuItem.Name.Value(item.Name),
		nullableStringAssign(&menuItem.Description, item.Description),
		menuItem.CategoryID.Value(item.CategoryID),
		menuItem.Price.Value(item.Price),
		menuItem.Currency.Value(item.Currency),
		menuItem.PrepMinutes.Value(item.PrepMinutes),
		menuItem.IsAvailable.Value(item.IsAvailable),
		menuItem.IsPopular.Value(item.IsPopular),
		nullableStringAssign(&menuItem.ImageURL, item.ImageURL),
		nullableStringAssign(&menuItem.ExternalURL, item.ExternalURL),
	}

	result, err := menuItemQuery.
		Where(menuItem.ID.Eq(item.ID)).
		UpdateSimple(assignments...)
	if err != nil {
		return repo.toMenuItemUpdateError(err)
	}
	if result.RowsAffected == 0 {
		return domainerrors.ErrMenuItemNotFound
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
	result, err := repo.q.MenuItemModel.WithContext(ctx).
		Where(repo.q.MenuItemModel.ID.Eq(id)).
		Update(repo.q.MenuItemModel.IsAvailable, isAvailable)
	if err != nil {
		return repo.toMenuItemUpdateError(err)
	}
	if result.RowsAffected == 0 {
		return domainerrors.ErrMenuItemNotFound
	}

	return nil
}

func (repo *menuRepository) DeleteMenuItem(ctx context.Context, merchantID, menuItemID uuid.UUID) error {
	return repo.withTransaction(func(transactionQuery *query.Query) error {
		if err := repo.lockMerchantProfileForMenuWrite(ctx, transactionQuery, merchantID); err != nil {
			return err //nolint:wrapcheck // already classified upstream when needed
		}

		menuItem := transactionQuery.MenuItemModel
		menuItemQuery := menuItem.WithContext(ctx)
		itemM, err := menuItemQuery.
			Clauses(clause.Locking{Strength: rowLockStrengthUpdate}).
			Where(
				menuItem.ID.Eq(menuItemID),
				menuItem.MerchantID.Eq(merchantID),
			).
			Take()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrMenuItemNotFound)
			}

			return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
		}

		if _, err := menuItemQuery.
			Where(menuItem.ID.Eq(menuItemID)).
			Delete(); err != nil {
			return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
		}

		if _, err := menuItemQuery.
			Where(
				menuItem.MerchantID.Eq(merchantID),
				menuItem.DisplayOrder.Gt(itemM.DisplayOrder),
			).
			Update(menuItem.DisplayOrder, gorm.Expr("display_order - 1")); err != nil {
			return repo.toMenuItemUpdateError(err)
		}

		return nil
	})
}

func (repo *menuRepository) getNextDisplayOrder(ctx context.Context, transactionQuery *query.Query, merchantID uuid.UUID) (int, error) {
	menuItem := transactionQuery.MenuItemModel
	menuItemQuery := menuItem.WithContext(ctx)
	itemM, err := menuItemQuery.
		Select(menuItem.DisplayOrder).
		Where(menuItem.MerchantID.Eq(merchantID)).
		Order(menuItem.DisplayOrder.Desc()).
		Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 1, nil
		}

		return 0, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	nextDisplayOrder := itemM.DisplayOrder + 1
	if nextDisplayOrder <= 0 {
		return 1, nil
	}

	return nextDisplayOrder, nil
}

func (repo *menuRepository) lockMerchantProfileForMenuWrite(ctx context.Context, transactionQuery *query.Query, merchantID uuid.UUID) error {
	merchantProfile := transactionQuery.MerchantProfileModel
	merchantProfileQuery := merchantProfile.WithContext(ctx)
	_, err := merchantProfileQuery.
		Select(merchantProfile.UserID).
		Clauses(clause.Locking{Strength: rowLockStrengthUpdate}).
		Where(merchantProfile.UserID.Eq(merchantID)).
		Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return replaceWithSourceStack(err, domainerrors.ErrMerchantNotFound)
		}

		return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	return nil
}

func (repo *menuRepository) toMenuItemCreateError(err error) error {
	if isUniqueConstraintViolation(err) {
		return replaceWithSourceStack(err, domainerrors.ErrMenuItemOrderConflict)
	}
	if isForeignKeyConstraintViolation(err) {
		return replaceWithSourceStack(err, domainerrors.ErrMenuItemCreateFailed)
	}
	if isNotNullConstraintViolation(err) {
		return replaceWithSourceStack(err, domainerrors.ErrMenuItemCreateFailed)
	}

	return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
}

func (repo *menuRepository) toMenuItemUpdateError(err error) error {
	if isUniqueConstraintViolation(err) {
		return replaceWithSourceStack(err, domainerrors.ErrMenuItemOrderConflict)
	}
	if isForeignKeyConstraintViolation(err) {
		return replaceWithSourceStack(err, domainerrors.ErrMenuItemUpdateFailed)
	}
	if isNotNullConstraintViolation(err) {
		return replaceWithSourceStack(err, domainerrors.ErrMenuItemUpdateFailed)
	}

	return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
}

func (repo *menuRepository) ReorderMenuItems(ctx context.Context, merchantID uuid.UUID, itemIDs []uuid.UUID) error {
	if len(itemIDs) == 0 {
		return nil
	}

	return repo.withTransaction(func(transactionQuery *query.Query) error {
		return repo.reorderMenuItemsTx(ctx, transactionQuery, merchantID, itemIDs) //nolint:wrapcheck // already classified upstream when needed
	})
}

func (repo *menuRepository) withTransaction(fn func(transactionQuery *query.Query) error) error {
	if err := repo.q.Transaction(fn); err != nil {
		if _, ok := errors.AsType[domainerrors.AppError](err); ok {
			return err //nolint:wrapcheck // preserve the original classified error
		}

		return replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	return nil
}

func (repo *menuRepository) reorderMenuItemsTx(ctx context.Context, transactionQuery *query.Query, merchantID uuid.UUID, itemIDs []uuid.UUID) error {
	if err := repo.lockMerchantProfileForMenuWrite(ctx, transactionQuery, merchantID); err != nil {
		return err //nolint:wrapcheck // already classified upstream when needed
	}

	scopedItemIDs, err := repo.listScopedMenuItemIDs(ctx, transactionQuery, merchantID)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped with persistence context
	}

	if err := repo.validateReorderMenuItems(ctx, transactionQuery, merchantID, scopedItemIDs, itemIDs); err != nil {
		return err //nolint:wrapcheck // already classified upstream when needed
	}

	if err := repo.bumpMenuItemDisplayOrders(ctx, transactionQuery, merchantID, len(itemIDs)); err != nil {
		return err //nolint:wrapcheck // already classified upstream when needed
	}

	return repo.applyMenuItemDisplayOrder(ctx, transactionQuery, itemIDs)
}

func (repo *menuRepository) listScopedMenuItemIDs(ctx context.Context, transactionQuery *query.Query, merchantID uuid.UUID) ([]uuid.UUID, error) {
	menuItem := transactionQuery.MenuItemModel
	menuItemQuery := menuItem.WithContext(ctx)
	scopedItems, err := menuItemQuery.
		Clauses(clause.Locking{Strength: rowLockStrengthUpdate}).
		Select(menuItem.ID).
		Where(menuItem.MerchantID.Eq(merchantID)).
		Order(menuItem.DisplayOrder.Asc()).
		Find()
	if err != nil {
		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	scopedItemIDs := make([]uuid.UUID, 0, len(scopedItems))
	for idx := range scopedItems {
		scopedItemIDs = append(scopedItemIDs, scopedItems[idx].ID)
	}

	return scopedItemIDs, nil
}

func (repo *menuRepository) validateReorderMenuItems(ctx context.Context, transactionQuery *query.Query, merchantID uuid.UUID, scopedItemIDs, itemIDs []uuid.UUID) error {
	providedItems, err := repo.listProvidedMenuItems(ctx, transactionQuery, itemIDs)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped with persistence context
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
		return domainerrors.ErrValidationFailed.WithDetails(reorderMenuItemsValidationMessage)
	}

	scopedItemSet := make(map[uuid.UUID]struct{}, len(scopedItemIDs))
	for idx := range scopedItemIDs {
		scopedItemSet[scopedItemIDs[idx]] = struct{}{}
	}

	for idx := range itemIDs {
		if _, exists := scopedItemSet[itemIDs[idx]]; !exists {
			return domainerrors.ErrValidationFailed.WithDetails(reorderMenuItemsValidationMessage)
		}
	}

	return nil
}

func (repo *menuRepository) listProvidedMenuItems(ctx context.Context, transactionQuery *query.Query, itemIDs []uuid.UUID) ([]menuItemMerchantRecord, error) {
	menuItem := transactionQuery.MenuItemModel
	menuItemQuery := menuItem.WithContext(ctx)
	ids := uuidToDriverValues(itemIDs)

	var providedItems []menuItemMerchantRecord
	if err := menuItemQuery.
		Clauses(clause.Locking{Strength: rowLockStrengthUpdate}).
		Select(menuItem.MerchantID).
		Where(menuItem.ID.In(ids...)).
		Scan(&providedItems); err != nil {
		return nil, replaceWithSourceStack(err, domainerrors.ErrPersistenceFailed)
	}

	return providedItems, nil
}

func (repo *menuRepository) bumpMenuItemDisplayOrders(ctx context.Context, transactionQuery *query.Query, merchantID uuid.UUID, itemCount int) error {
	menuItem := transactionQuery.MenuItemModel
	menuItemQuery := menuItem.WithContext(ctx)
	if _, err := menuItemQuery.
		Where(menuItem.MerchantID.Eq(merchantID)).
		Update(menuItem.DisplayOrder, gorm.Expr("display_order + ?", itemCount)); err != nil {
		return repo.toMenuItemUpdateError(err)
	}

	return nil
}

func (repo *menuRepository) applyMenuItemDisplayOrder(ctx context.Context, transactionQuery *query.Query, itemIDs []uuid.UUID) error {
	menuItem := transactionQuery.MenuItemModel
	menuItemQuery := menuItem.WithContext(ctx)
	for index, itemID := range itemIDs {
		if _, err := menuItemQuery.
			Where(menuItem.ID.Eq(itemID)).
			Update(menuItem.DisplayOrder, index+1); err != nil {
			return repo.toMenuItemUpdateError(err)
		}
	}

	return nil
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
		CategoryID:   data.CategoryID,
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
		CategoryID:   data.CategoryID,
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

func nullableStringAssign(column *field.String, value *string) field.AssignExpr {
	if value == nil {
		return column.Null()
	}

	return column.Value(*value)
}

func uuidToDriverValues(values []uuid.UUID) []driver.Valuer {
	driverValues := make([]driver.Valuer, len(values))
	for idx := range values {
		driverValues[idx] = values[idx]
	}

	return driverValues
}
