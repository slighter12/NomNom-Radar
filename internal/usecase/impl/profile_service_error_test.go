package impl

import (
	"context"
	"errors"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProfileService_GetProfile_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
	})

	_, err := fx.service.GetProfile(ctx, userID)

	assert.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrNotFound)
}

func TestProfileService_GetProfile_FindError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, expectedErr)
	})

	user, err := fx.service.GetProfile(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_UpdateMerchantProfile_NoProfile(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateMerchantProfileInput{
		StoreName:        nil,
		StoreDescription: nil,
	}

	existingUser := &entity.User{
		ID: userID,
		// MerchantProfile intentionally nil to trigger validation
	}

	fx.onExecute(ctx, domainerrors.ErrValidationFailed.WithDetails("user does not have a merchant profile"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrValidationFailed))
}

func TestProfileService_UpdateMerchantProfile_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateMerchantProfileInput{}

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestProfileService_UpdateMerchantProfile_FindError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateMerchantProfileInput{}

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, expectedErr)
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_UpdateMerchantProfile_UpdateError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	storeName := "New Name"
	input := &usecase.UpdateMerchantProfileInput{
		StoreName: &storeName,
	}

	existingUser := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:    userID,
			StoreName: "Old Name",
		},
	}

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(expectedErr)
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_UpdateUserProfile_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateUserProfileInput{}

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestProfileService_UpdateUserProfile_NoProfile(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateUserProfileInput{}

	existingUser := &entity.User{
		ID:          userID,
		UserProfile: nil, // No user profile
	}

	fx.onExecute(ctx, domainerrors.ErrValidationFailed.WithDetails("user does not have a user profile"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrValidationFailed))
}

func TestProfileService_UpdateUserProfile_FindError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateUserProfileInput{}

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, expectedErr)
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_UpdateUserProfile_UpdateError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	points := 200
	input := &usecase.UpdateUserProfileInput{
		LoyaltyPoints: &points,
	}

	existingUser := &entity.User{
		ID: userID,
		UserProfile: &entity.UserProfile{
			UserID:        userID,
			LoyaltyPoints: 100,
		},
	}

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(expectedErr)
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_SwitchToMerchant_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL-123",
	}

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestProfileService_SwitchToMerchant_FindError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL-123",
	}

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, expectedErr)
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_SwitchToMerchant_AlreadyMerchant(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL-123",
	}

	existingUser := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:    userID,
			StoreName: "Existing Store",
		},
	}

	fx.onExecute(ctx, domainerrors.ErrConflict.WithDetails("user already has a merchant profile"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrConflict))
}

func TestProfileService_SwitchToMerchant_UpdateError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL-123",
	}

	existingUser := &entity.User{
		ID:              userID,
		MerchantProfile: nil,
	}

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(expectedErr)
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestProfileService_GetUserRole_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, roles)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestProfileService_GetUserRole_FindError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, expectedErr)
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, roles)
	assert.ErrorIs(t, err, expectedErr)
}
