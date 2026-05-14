package impl

import (
	"context"
	"errors"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func executeProfileDiscoveryTx(
	t *testing.T,
	fx *profileServiceFixtures,
	ctx context.Context,
	setupMocks func(factory *mockRepo.MockRepositoryFactory),
) {
	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			setupMocks(mockFactory)

			return fn(mockFactory)
		})
}

func TestProfileService_GetMerchantDiscoveryProfile_Success(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	hubID := uuid.New()
	category := &entity.DiscoveryCategory{ID: categoryID, Slug: "meal", Status: entity.DiscoveryStatusActive}
	subcategory := &entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: categoryID, Slug: "grill", Status: entity.DiscoveryStatusActive}
	hub := &entity.Hub{ID: hubID, Slug: "night-market", Status: entity.DiscoveryStatusActive}
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:                 userID,
			VerificationStatus:     entity.MerchantVerificationStatusVerified,
			DiscoveryCategoryID:    &categoryID,
			DiscoverySubcategoryID: &subcategoryID,
			ActiveHubID:            &hubID,
			IsPublic:               true,
		},
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockAddressRepo.EXPECT().
			FindActiveAddressesByOwner(ctx, userID, entity.OwnerTypeMerchantProfile).
			Return([]*entity.Address{{ID: uuid.New(), OwnerID: userID, OwnerType: entity.OwnerTypeMerchantProfile, IsPrimary: true, IsActive: true}}, nil)
		mockDiscoveryRepo.EXPECT().FindCategoryByID(ctx, categoryID).Return(category, nil)
		mockDiscoveryRepo.EXPECT().FindSubcategoryByID(ctx, subcategoryID).Return(subcategory, nil)
		mockDiscoveryRepo.EXPECT().FindHubByID(ctx, hubID).Return(hub, nil)
	})

	result, err := fx.service.GetMerchantDiscoveryProfile(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, &categoryID, result.DiscoveryCategoryID)
	assert.Equal(t, &subcategoryID, result.DiscoverySubcategoryID)
	assert.Equal(t, &hubID, result.ActiveHubID)
	assert.True(t, result.IsPublic)
	assert.True(t, result.IsVerified)
	assert.True(t, result.HasActivePrimaryLocation)
	assert.Equal(t, category, result.DiscoveryCategory)
	assert.Equal(t, subcategory, result.DiscoverySubcategory)
	assert.Equal(t, hub, result.ActiveHub)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_EnablePublicSuccess(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	hubID := uuid.New()
	isPublic := true
	category := &entity.DiscoveryCategory{ID: categoryID, Slug: "meal", Status: entity.DiscoveryStatusActive}
	subcategory := &entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: categoryID, Slug: "grill", Status: entity.DiscoveryStatusActive}
	hub := &entity.Hub{ID: hubID, Slug: "night-market", Status: entity.DiscoveryStatusActive}
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:             userID,
			VerificationStatus: entity.MerchantVerificationStatusVerified,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		DiscoveryCategoryID:    usecase.OptionalUUIDUpdate{IsSet: true, Value: &categoryID},
		DiscoverySubcategoryID: usecase.OptionalUUIDUpdate{IsSet: true, Value: &subcategoryID},
		ActiveHubID:            usecase.OptionalUUIDUpdate{IsSet: true, Value: &hubID},
		IsPublic:               &isPublic,
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockDiscoveryRepo.EXPECT().FindCategoryByID(ctx, categoryID).Return(category, nil).Twice()
		mockDiscoveryRepo.EXPECT().FindSubcategoryByID(ctx, subcategoryID).Return(subcategory, nil).Twice()
		mockDiscoveryRepo.EXPECT().FindHubByID(ctx, hubID).Return(hub, nil).Twice()
		mockAddressRepo.EXPECT().
			FindActiveAddressesByOwner(ctx, userID, entity.OwnerTypeMerchantProfile).
			Return([]*entity.Address{{ID: uuid.New(), OwnerID: userID, OwnerType: entity.OwnerTypeMerchantProfile, IsPrimary: true, IsActive: true}}, nil)
		mockUserRepo.EXPECT().
			Update(ctx, mock.AnythingOfType("*entity.User")).
			Run(func(_ context.Context, updated *entity.User) {
				assert.Equal(t, &categoryID, updated.MerchantProfile.DiscoveryCategoryID)
				assert.Equal(t, &subcategoryID, updated.MerchantProfile.DiscoverySubcategoryID)
				assert.Equal(t, &hubID, updated.MerchantProfile.ActiveHubID)
				assert.True(t, updated.MerchantProfile.IsPublic)
			}).
			Return(nil)
	})

	result, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.NoError(t, err)
	assert.True(t, result.IsPublic)
	assert.Equal(t, category, result.DiscoveryCategory)
	assert.Equal(t, subcategory, result.DiscoverySubcategory)
	assert.Equal(t, hub, result.ActiveHub)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_ClearActiveHub(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	hubID := uuid.New()
	isPublic := false
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:             userID,
			VerificationStatus: entity.MerchantVerificationStatusUnverified,
			ActiveHubID:        &hubID,
			IsPublic:           true,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		ActiveHubID: usecase.OptionalUUIDUpdate{IsSet: true, Value: nil},
		IsPublic:    &isPublic,
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockAddressRepo.EXPECT().
			FindActiveAddressesByOwner(ctx, userID, entity.OwnerTypeMerchantProfile).
			Return(nil, nil)
		mockUserRepo.EXPECT().
			Update(ctx, mock.AnythingOfType("*entity.User")).
			Run(func(_ context.Context, updated *entity.User) {
				assert.Nil(t, updated.MerchantProfile.ActiveHubID)
				assert.False(t, updated.MerchantProfile.IsPublic)
			}).
			Return(nil)
	})

	result, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.NoError(t, err)
	assert.Nil(t, result.ActiveHubID)
	assert.False(t, result.IsPublic)
	assert.False(t, result.HasActivePrimaryLocation)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_AllowsDisablingPublicWithInactiveExistingValues(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	hubID := uuid.New()
	isPublic := false
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:                 userID,
			VerificationStatus:     entity.MerchantVerificationStatusVerified,
			DiscoveryCategoryID:    &categoryID,
			DiscoverySubcategoryID: &subcategoryID,
			ActiveHubID:            &hubID,
			IsPublic:               true,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		IsPublic: &isPublic,
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockAddressRepo.EXPECT().
			FindActiveAddressesByOwner(ctx, userID, entity.OwnerTypeMerchantProfile).
			Return(nil, nil)
		updateCall := mockUserRepo.EXPECT().
			Update(ctx, mock.AnythingOfType("*entity.User")).
			Run(func(_ context.Context, updated *entity.User) {
				assert.False(t, updated.MerchantProfile.IsPublic)
			}).
			Return(nil)
		mockDiscoveryRepo.EXPECT().
			FindCategoryByID(ctx, categoryID).
			Return(&entity.DiscoveryCategory{ID: categoryID, Status: entity.DiscoveryStatusInactive}, nil).
			NotBefore(updateCall.Call)
		mockDiscoveryRepo.EXPECT().
			FindSubcategoryByID(ctx, subcategoryID).
			Return(&entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: categoryID, Status: entity.DiscoveryStatusInactive}, nil).
			NotBefore(updateCall.Call)
		mockDiscoveryRepo.EXPECT().
			FindHubByID(ctx, hubID).
			Return(&entity.Hub{ID: hubID, Status: entity.DiscoveryStatusInactive}, nil).
			NotBefore(updateCall.Call)
	})

	result, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.NoError(t, err)
	assert.False(t, result.IsPublic)
	assert.Equal(t, entity.DiscoveryStatusInactive, result.DiscoveryCategory.Status)
	assert.Equal(t, entity.DiscoveryStatusInactive, result.DiscoverySubcategory.Status)
	assert.Equal(t, entity.DiscoveryStatusInactive, result.ActiveHub.Status)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_RejectsSelectedInactiveHubWhenPrivate(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	hubID := uuid.New()
	isPublic := false
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:             userID,
			VerificationStatus: entity.MerchantVerificationStatusVerified,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		ActiveHubID: usecase.OptionalUUIDUpdate{IsSet: true, Value: &hubID},
		IsPublic:    &isPublic,
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockDiscoveryRepo.EXPECT().
			FindHubByID(ctx, hubID).
			Return(&entity.Hub{ID: hubID, Status: entity.DiscoveryStatusInactive}, nil)
	})

	_, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_RejectsMismatchedSubcategory(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	otherCategoryID := uuid.New()
	subcategoryID := uuid.New()
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:             userID,
			VerificationStatus: entity.MerchantVerificationStatusVerified,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		DiscoveryCategoryID:    usecase.OptionalUUIDUpdate{IsSet: true, Value: &categoryID},
		DiscoverySubcategoryID: usecase.OptionalUUIDUpdate{IsSet: true, Value: &subcategoryID},
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockDiscoveryRepo.EXPECT().
			FindCategoryByID(ctx, categoryID).
			Return(&entity.DiscoveryCategory{ID: categoryID, Status: entity.DiscoveryStatusActive}, nil)
		mockDiscoveryRepo.EXPECT().
			FindSubcategoryByID(ctx, subcategoryID).
			Return(&entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: otherCategoryID, Status: entity.DiscoveryStatusActive}, nil)
	})

	_, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
	assert.Contains(t, err.Error(), "輸入資料驗證失敗")
}

func TestProfileService_UpdateMerchantDiscoveryProfile_RejectsUnverifiedPublicMerchant(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	isPublic := true
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:             userID,
			VerificationStatus: entity.MerchantVerificationStatusUnverified,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		DiscoveryCategoryID:    usecase.OptionalUUIDUpdate{IsSet: true, Value: &categoryID},
		DiscoverySubcategoryID: usecase.OptionalUUIDUpdate{IsSet: true, Value: &subcategoryID},
		IsPublic:               &isPublic,
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockDiscoveryRepo.EXPECT().
			FindCategoryByID(ctx, categoryID).
			Return(&entity.DiscoveryCategory{ID: categoryID, Status: entity.DiscoveryStatusActive}, nil)
		mockDiscoveryRepo.EXPECT().
			FindSubcategoryByID(ctx, subcategoryID).
			Return(&entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: categoryID, Status: entity.DiscoveryStatusActive}, nil)
		mockAddressRepo.EXPECT().
			FindActiveAddressesByOwner(ctx, userID, entity.OwnerTypeMerchantProfile).
			Return([]*entity.Address{{ID: uuid.New(), OwnerID: userID, OwnerType: entity.OwnerTypeMerchantProfile, IsPrimary: true, IsActive: true}}, nil)
	})

	_, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_RejectsPublicWithoutActivePrimaryLocation(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	isPublic := true
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:             userID,
			VerificationStatus: entity.MerchantVerificationStatusVerified,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		DiscoveryCategoryID:    usecase.OptionalUUIDUpdate{IsSet: true, Value: &categoryID},
		DiscoverySubcategoryID: usecase.OptionalUUIDUpdate{IsSet: true, Value: &subcategoryID},
		IsPublic:               &isPublic,
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockDiscoveryRepo.EXPECT().
			FindCategoryByID(ctx, categoryID).
			Return(&entity.DiscoveryCategory{ID: categoryID, Status: entity.DiscoveryStatusActive}, nil)
		mockDiscoveryRepo.EXPECT().
			FindSubcategoryByID(ctx, subcategoryID).
			Return(&entity.DiscoverySubcategory{ID: subcategoryID, CategoryID: categoryID, Status: entity.DiscoveryStatusActive}, nil)
		mockAddressRepo.EXPECT().
			FindActiveAddressesByOwner(ctx, userID, entity.OwnerTypeMerchantProfile).
			Return(nil, nil)
	})

	_, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrValidationFailed)
}

func TestProfileService_UpdateMerchantDiscoveryProfile_PropagatesInvalidDiscoveryReferences(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	categoryID := uuid.New()
	user := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID: userID,
		},
	}
	input := &usecase.UpdateMerchantDiscoveryProfileInput{
		DiscoveryCategoryID: usecase.OptionalUUIDUpdate{IsSet: true, Value: &categoryID},
	}

	executeProfileDiscoveryTx(t, fx, ctx, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockDiscoveryRepo := mockRepo.NewMockDiscoveryRepository(t)
		mockAddressRepo := mockRepo.NewMockAddressRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().DiscoveryRepo().Return(mockDiscoveryRepo)
		factory.EXPECT().AddressRepo().Return(mockAddressRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockDiscoveryRepo.EXPECT().FindCategoryByID(ctx, categoryID).Return(nil, domainerrors.ErrDiscoveryCategoryNotFound)
	})

	_, err := fx.service.UpdateMerchantDiscoveryProfile(ctx, userID, input)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrDiscoveryCategoryNotFound))
}
