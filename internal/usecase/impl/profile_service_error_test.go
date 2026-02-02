package impl

import (
	"context"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProfileService_GetProfile_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, repository.ErrUserNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	_, err := fx.service.GetProfile(ctx, userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestProfileService_GetProfile_FindError(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, errors.New("db error"))
	})

	user, err := fx.service.GetProfile(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "failed to find user")
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

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrValidationFailed, "user does not have a merchant profile"), func(factory *mockRepo.MockRepositoryFactory) {
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

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, errors.New("db error"))
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find user")
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to update merchant profile"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(errors.New("db error"))
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update merchant profile")
}

func TestProfileService_UpdateUserProfile_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.UpdateUserProfileInput{}

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
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

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrValidationFailed, "user does not have a user profile"), func(factory *mockRepo.MockRepositoryFactory) {
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, errors.New("db error"))
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find user")
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to update user profile"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(errors.New("db error"))
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update user profile")
}

func TestProfileService_SwitchToMerchant_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL-123",
	}

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, errors.New("db error"))
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find user")
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

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrConflict, "user already has a merchant profile"), func(factory *mockRepo.MockRepositoryFactory) {
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to create merchant profile"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(errors.New("db error"))
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create merchant profile")
}

func TestProfileService_GetUserRole_NotFound(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
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

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, errors.New("db error"))
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, roles)
	assert.Contains(t, err.Error(), "failed to find user")
}
