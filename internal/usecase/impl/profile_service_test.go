package impl

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProfileService_GetProfile_Success(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewProfileService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	expectedUser := &entity.User{
		ID:    userID,
		Email: "test@example.com",
		Name:  "Test User",
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(expectedUser, nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	user, err := service.GetProfile(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, expectedUser, user)
}

func TestProfileService_UpdateUserProfile_Success(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewProfileService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	points := 100
	input := &usecase.UpdateUserProfileInput{
		LoyaltyPoints: &points,
	}

	existingUser := &entity.User{
		ID: userID,
		UserProfile: &entity.UserProfile{
			UserID:        userID,
			LoyaltyPoints: 0,
		},
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
			mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	err := service.UpdateUserProfile(ctx, userID, input)

	require.NoError(t, err)
	assert.Equal(t, points, existingUser.UserProfile.LoyaltyPoints)
}

func TestProfileService_SwitchToMerchant_Success(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewProfileService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL123",
	}

	existingUser := &entity.User{
		ID: userID,
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
			mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	err := service.SwitchToMerchant(ctx, userID, input)

	require.NoError(t, err)
	assert.NotNil(t, existingUser.MerchantProfile)
	assert.Equal(t, input.StoreName, existingUser.MerchantProfile.StoreName)
}

func TestProfileService_GetUserRole(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewProfileService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{
		ID:              userID,
		UserProfile:     &entity.UserProfile{},
		MerchantProfile: &entity.MerchantProfile{},
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	roles, err := service.GetUserRole(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, roles, 2)
	assert.Contains(t, roles, "user")
	assert.Contains(t, roles, "merchant")
}

func TestProfileService_UpdateMerchantProfile_NoProfile(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewProfileService(txManager, logger)

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

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)

			_ = fn(mockFactory)
		}).
		Return(assert.AnError)

	err := service.UpdateMerchantProfile(ctx, userID, input)

	assert.Error(t, err)
}

func TestProfileService_GetProfile_NotFound(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewProfileService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)

			_ = fn(mockFactory)
		}).
		Return(repository.ErrUserNotFound)

	_, err := service.GetProfile(ctx, userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}
