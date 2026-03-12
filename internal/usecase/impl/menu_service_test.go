package impl

import (
	"context"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type menuRepositoryStub struct {
	createMenuItemFunc             func(ctx context.Context, item *entity.MenuItem) error
	findMenuItemByIDFunc           func(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error)
	listActiveMenuItemIDsFunc      func(ctx context.Context, merchantID uuid.UUID) ([]uuid.UUID, error)
	listMenuItemsByMerchantFunc    func(ctx context.Context, merchantID uuid.UUID, filter repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error)
	updateMenuItemFunc             func(ctx context.Context, item *entity.MenuItem) error
	updateMenuItemAvailabilityFunc func(ctx context.Context, id uuid.UUID, isAvailable bool) error
	deleteMenuItemFunc             func(ctx context.Context, merchantID, id uuid.UUID) error
	reorderMenuItemsFunc           func(ctx context.Context, merchantID uuid.UUID, itemIDs []uuid.UUID) error
}

func (s *menuRepositoryStub) CreateMenuItem(ctx context.Context, item *entity.MenuItem) error {
	return s.createMenuItemFunc(ctx, item)
}

func (s *menuRepositoryStub) FindMenuItemByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	return s.findMenuItemByIDFunc(ctx, id)
}

func (s *menuRepositoryStub) ListActiveMenuItemIDsByMerchant(ctx context.Context, merchantID uuid.UUID) ([]uuid.UUID, error) {
	return s.listActiveMenuItemIDsFunc(ctx, merchantID)
}

func (s *menuRepositoryStub) ListMenuItemsByMerchant(ctx context.Context, merchantID uuid.UUID, filter repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
	return s.listMenuItemsByMerchantFunc(ctx, merchantID, filter)
}

func (s *menuRepositoryStub) UpdateMenuItem(ctx context.Context, item *entity.MenuItem) error {
	return s.updateMenuItemFunc(ctx, item)
}

func (s *menuRepositoryStub) UpdateMenuItemAvailability(ctx context.Context, id uuid.UUID, isAvailable bool) error {
	return s.updateMenuItemAvailabilityFunc(ctx, id, isAvailable)
}

func (s *menuRepositoryStub) DeleteMenuItem(ctx context.Context, merchantID, id uuid.UUID) error {
	return s.deleteMenuItemFunc(ctx, merchantID, id)
}

func (s *menuRepositoryStub) ReorderMenuItems(ctx context.Context, merchantID uuid.UUID, itemIDs []uuid.UUID) error {
	return s.reorderMenuItemsFunc(ctx, merchantID, itemIDs)
}

type userRepositoryStub struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

func (s *userRepositoryStub) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	if s.findByIDFunc == nil {
		return nil, nil
	}

	return s.findByIDFunc(ctx, id)
}

func (s *userRepositoryStub) AcquireSessionMutex(context.Context, uuid.UUID) error {
	return nil
}

func (s *userRepositoryStub) FindByEmail(context.Context, string) (*entity.User, error) {
	return nil, nil
}

func (s *userRepositoryStub) Create(context.Context, *entity.User) error {
	return nil
}

func (s *userRepositoryStub) Update(context.Context, *entity.User) error {
	return nil
}

func TestMenuService_CreateMenuItem_Success(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	now := time.Now()

	repo := &menuRepositoryStub{
		createMenuItemFunc: func(_ context.Context, item *entity.MenuItem) error {
			item.DisplayOrder = 3
			item.CreatedAt = now
			item.UpdatedAt = now

			return nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	item, err := service.CreateMenuItem(ctx, merchantID, &usecase.CreateMenuItemInput{
		Name:        "  Beef Noodles  ",
		Category:    "main",
		Price:       180,
		Currency:    "twd",
		PrepMinutes: 15,
	})

	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, merchantID, item.MerchantID)
	assert.Equal(t, "Beef Noodles", item.Name)
	assert.Equal(t, entity.MenuCategoryMain, item.Category)
	assert.Equal(t, entity.CurrencyTWD, item.Currency)
	assert.Equal(t, 3, item.DisplayOrder)
}

func TestMenuService_CreateMenuItem_InvalidCategory(t *testing.T) {
	service := NewMenuService(MenuServiceParams{
		MenuRepo: &menuRepositoryStub{
			createMenuItemFunc: func(_ context.Context, _ *entity.MenuItem) error { return nil },
		},
	})

	item, err := service.CreateMenuItem(context.Background(), uuid.New(), &usecase.CreateMenuItemInput{
		Name:        "Combo",
		Category:    "combo",
		Price:       120,
		Currency:    "TWD",
		PrepMinutes: 10,
	})

	require.Error(t, err)
	assert.Nil(t, item)
	assert.ErrorIs(t, err, domainerrors.ErrInvalidMenuCategory)
}

func TestMenuService_UpdateMenuItem_PreservesDisplayOrder(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	itemID := uuid.New()
	existingItem := &entity.MenuItem{
		ID:           itemID,
		MerchantID:   merchantID,
		Name:         "Old Name",
		Category:     entity.MenuCategoryDrink,
		Currency:     entity.CurrencyTWD,
		Price:        60,
		PrepMinutes:  5,
		DisplayOrder: 4,
	}

	repo := &menuRepositoryStub{
		findMenuItemByIDFunc: func(_ context.Context, id uuid.UUID) (*entity.MenuItem, error) {
			assert.Equal(t, itemID, id)

			return existingItem, nil
		},
		updateMenuItemFunc: func(_ context.Context, item *entity.MenuItem) error {
			assert.Equal(t, 4, item.DisplayOrder)
			assert.Equal(t, "New Name", item.Name)

			return nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	item, err := service.UpdateMenuItem(ctx, merchantID, itemID, &usecase.UpdateMenuItemInput{
		Name:        "New Name",
		Category:    "drink",
		Price:       70,
		Currency:    "TWD",
		PrepMinutes: 6,
		IsAvailable: true,
		IsPopular:   false,
	})

	require.NoError(t, err)
	assert.Equal(t, 4, item.DisplayOrder)
	assert.Equal(t, "New Name", item.Name)
}

func TestMenuService_ReorderMenuItems_RequiresCompleteSnapshot(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	itemA := uuid.New()
	itemB := uuid.New()
	itemC := uuid.New()
	reorderCalled := false

	repo := &menuRepositoryStub{
		listActiveMenuItemIDsFunc: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			assert.Equal(t, merchantID, id)

			return []uuid.UUID{itemA, itemB, itemC}, nil
		},
		reorderMenuItemsFunc: func(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
			reorderCalled = true

			return nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	result, err := service.ReorderMenuItems(ctx, merchantID, &usecase.ReorderMenuItemsInput{
		ItemIDs: []uuid.UUID{itemB, itemA},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "all active menu item ids")
	assert.False(t, reorderCalled)
}

func TestMenuService_ReorderMenuItems_Success(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	itemA := uuid.New()
	itemB := uuid.New()
	itemC := uuid.New()
	var reorderedIDs []uuid.UUID

	repo := &menuRepositoryStub{
		listActiveMenuItemIDsFunc: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			assert.Equal(t, merchantID, id)

			return []uuid.UUID{itemA, itemB, itemC}, nil
		},
		reorderMenuItemsFunc: func(_ context.Context, id uuid.UUID, itemIDs []uuid.UUID) error {
			assert.Equal(t, merchantID, id)
			reorderedIDs = append([]uuid.UUID{}, itemIDs...)

			return nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	result, err := service.ReorderMenuItems(ctx, merchantID, &usecase.ReorderMenuItemsInput{
		ItemIDs: []uuid.UUID{itemC, itemA, itemB},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.UpdatedCount)
	assert.Equal(t, []uuid.UUID{itemC, itemA, itemB}, reorderedIDs)
}

func TestMenuService_GetPublicMerchantMenu_Success(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	expectedItems := []*entity.MenuItem{
		{ID: uuid.New(), MerchantID: merchantID, Name: "Beef Noodles", IsAvailable: true},
	}

	menuRepo := &menuRepositoryStub{
		listMenuItemsByMerchantFunc: func(_ context.Context, id uuid.UUID, filter repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
			assert.Equal(t, merchantID, id)
			require.NotNil(t, filter.IsAvailable)
			assert.True(t, *filter.IsAvailable)
			require.NotNil(t, filter.Category)
			assert.Equal(t, entity.MenuCategoryMain, *filter.Category)
			assert.Equal(t, "beef", filter.Keyword)
			assert.Equal(t, 20, filter.Limit)
			assert.Equal(t, 0, filter.Offset)

			return expectedItems, 1, nil
		},
	}
	userRepo := &userRepositoryStub{
		findByIDFunc: func(_ context.Context, id uuid.UUID) (*entity.User, error) {
			assert.Equal(t, merchantID, id)

			return &entity.User{
				ID:              merchantID,
				MerchantProfile: &entity.MerchantProfile{UserID: merchantID},
			}, nil
		},
	}

	service := NewMenuService(MenuServiceParams{
		MenuRepo: menuRepo,
		UserRepo: userRepo,
	})

	result, err := service.GetPublicMerchantMenu(ctx, merchantID, &usecase.ListMerchantMenuItemsInput{
		Category: "  main  ",
		Keyword:  "  beef  ",
		Page:     1,
		PageSize: 20,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedItems, result.Items)
	assert.Equal(t, 1, result.Pagination.Page)
	assert.Equal(t, 20, result.Pagination.PageSize)
	assert.EqualValues(t, 1, result.Pagination.Total)
}

func TestMenuService_GetPublicMerchantMenu_MerchantNotFound(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		buildUser   func(merchantID uuid.UUID) *entity.User
		findByIDErr error
	}{
		{
			name:        "missing user",
			findByIDErr: repository.ErrUserNotFound,
		},
		{
			name: "user without merchant profile",
			buildUser: func(merchantID uuid.UUID) *entity.User {
				return &entity.User{ID: merchantID}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merchantID := uuid.New()
			menuRepoCalled := false

			menuRepo := &menuRepositoryStub{
				listMenuItemsByMerchantFunc: func(context.Context, uuid.UUID, repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
					menuRepoCalled = true
					t.Fatal("menu repository should not be called when merchant is invalid")

					return nil, 0, nil
				},
			}
			userRepo := &userRepositoryStub{
				findByIDFunc: func(_ context.Context, id uuid.UUID) (*entity.User, error) {
					assert.Equal(t, merchantID, id)

					if tt.buildUser != nil {
						return tt.buildUser(merchantID), tt.findByIDErr
					}

					return nil, tt.findByIDErr
				},
			}

			service := NewMenuService(MenuServiceParams{
				MenuRepo: menuRepo,
				UserRepo: userRepo,
			})

			result, err := service.GetPublicMerchantMenu(ctx, merchantID, &usecase.ListMerchantMenuItemsInput{
				Page:     1,
				PageSize: 20,
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domainerrors.ErrMerchantNotFound)
			assert.False(t, menuRepoCalled)
		})
	}
}

func TestMenuService_ListMerchantMenuItems_Success(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	isAvailable := true
	expectedItems := []*entity.MenuItem{
		{ID: uuid.New(), MerchantID: merchantID, Name: "Beef Noodles"},
	}

	repo := &menuRepositoryStub{
		listMenuItemsByMerchantFunc: func(_ context.Context, id uuid.UUID, filter repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
			assert.Equal(t, merchantID, id)
			require.NotNil(t, filter.IsAvailable)
			assert.True(t, *filter.IsAvailable)
			require.NotNil(t, filter.Category)
			assert.Equal(t, entity.MenuCategoryMain, *filter.Category)
			assert.Equal(t, "beef", filter.Keyword)
			assert.Equal(t, 5, filter.Limit)
			assert.Equal(t, 5, filter.Offset)

			return expectedItems, 7, nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	result, err := service.ListMerchantMenuItems(ctx, merchantID, &usecase.ListMerchantMenuItemsInput{
		Category:    "  main  ",
		IsAvailable: &isAvailable,
		Keyword:     "  beef  ",
		Page:        2,
		PageSize:    5,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedItems, result.Items)
	assert.Equal(t, 2, result.Pagination.Page)
	assert.Equal(t, 5, result.Pagination.PageSize)
	assert.EqualValues(t, 7, result.Pagination.Total)
}

func TestMenuService_ListMerchantMenuItems_InvalidCategory(t *testing.T) {
	service := NewMenuService(MenuServiceParams{
		MenuRepo: &menuRepositoryStub{
			listMenuItemsByMerchantFunc: func(context.Context, uuid.UUID, repository.MenuItemListFilter) ([]*entity.MenuItem, int64, error) {
				t.Fatal("repository should not be called for invalid category")

				return nil, 0, nil
			},
		},
	})

	result, err := service.ListMerchantMenuItems(context.Background(), uuid.New(), &usecase.ListMerchantMenuItemsInput{
		Category: "combo",
		Page:     1,
		PageSize: 10,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainerrors.ErrInvalidMenuCategory)
}

func TestMenuService_UpdateMenuItemStatus_Success(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	itemID := uuid.New()
	findCalls := 0

	repo := &menuRepositoryStub{
		findMenuItemByIDFunc: func(_ context.Context, id uuid.UUID) (*entity.MenuItem, error) {
			assert.Equal(t, itemID, id)
			findCalls++
			if findCalls == 1 {
				return &entity.MenuItem{ID: itemID, MerchantID: merchantID, IsAvailable: true}, nil
			}

			return &entity.MenuItem{ID: itemID, MerchantID: merchantID, IsAvailable: false}, nil
		},
		updateMenuItemAvailabilityFunc: func(_ context.Context, id uuid.UUID, isAvailable bool) error {
			assert.Equal(t, itemID, id)
			assert.False(t, isAvailable)

			return nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	item, err := service.UpdateMenuItemStatus(ctx, merchantID, itemID, false)

	require.NoError(t, err)
	require.NotNil(t, item)
	assert.False(t, item.IsAvailable)
	assert.Equal(t, 2, findCalls)
}

func TestMenuService_UpdateMenuItemStatus_NotFound(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	itemID := uuid.New()

	repo := &menuRepositoryStub{
		findMenuItemByIDFunc: func(_ context.Context, id uuid.UUID) (*entity.MenuItem, error) {
			assert.Equal(t, itemID, id)

			return &entity.MenuItem{ID: itemID, MerchantID: merchantID, IsAvailable: true}, nil
		},
		updateMenuItemAvailabilityFunc: func(_ context.Context, id uuid.UUID, isAvailable bool) error {
			assert.Equal(t, itemID, id)
			assert.False(t, isAvailable)

			return repository.ErrMenuItemNotFound
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})
	item, err := service.UpdateMenuItemStatus(ctx, merchantID, itemID, false)

	require.Error(t, err)
	assert.Nil(t, item)
	assert.ErrorIs(t, err, domainerrors.ErrMenuItemNotFound)
}

func TestMenuService_DeleteMenuItem_Success(t *testing.T) {
	ctx := context.Background()
	merchantID := uuid.New()
	itemID := uuid.New()

	repo := &menuRepositoryStub{
		findMenuItemByIDFunc: func(_ context.Context, id uuid.UUID) (*entity.MenuItem, error) {
			assert.Equal(t, itemID, id)

			return &entity.MenuItem{ID: itemID, MerchantID: merchantID}, nil
		},
		deleteMenuItemFunc: func(_ context.Context, ownerID, id uuid.UUID) error {
			assert.Equal(t, merchantID, ownerID)
			assert.Equal(t, itemID, id)

			return nil
		},
	}

	service := NewMenuService(MenuServiceParams{MenuRepo: repo})

	require.NoError(t, service.DeleteMenuItem(ctx, merchantID, itemID))
}
