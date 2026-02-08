package impl

import (
	"context"
	"io"
	"log/slog"
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

// profileServiceFixtures holds all test dependencies for profile service tests.
type profileServiceFixtures struct {
	t         *testing.T
	service   usecase.ProfileUsecase
	txManager *mockRepo.MockTransactionManager
}

func createTestProfileService(t *testing.T) *profileServiceFixtures {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewProfileService(txManager, logger)

	return &profileServiceFixtures{
		t:         t,
		service:   service,
		txManager: txManager,
	}
}

// onExecute is a helper method to reduce boilerplate for mocking txManager.Execute.
func (fx *profileServiceFixtures) onExecute(ctx context.Context, returnErr error, setupMocks func(factory *mockRepo.MockRepositoryFactory)) {
	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(fx.t)
			setupMocks(mockFactory)
			_ = fn(mockFactory)
		}).
		Return(returnErr)
}

func TestProfileService_GetProfile_Success(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	expectedUser := &entity.User{
		ID:    userID,
		Email: "test@example.com",
		Name:  "Test User",
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(expectedUser, nil)
	})

	user, err := fx.service.GetProfile(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, expectedUser, user)
}

func TestProfileService_UpdateUserProfile_Success(t *testing.T) {
	fx := createTestProfileService(t)

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

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(nil)
	})

	err := fx.service.UpdateUserProfile(ctx, userID, input)

	require.NoError(t, err)
	assert.Equal(t, points, existingUser.UserProfile.LoyaltyPoints)
}

func TestProfileService_SwitchToMerchant_Success(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.SwitchToMerchantInput{
		StoreName:       "Test Store",
		BusinessLicense: "BL123",
	}

	existingUser := &entity.User{
		ID: userID,
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(nil)
	})

	err := fx.service.SwitchToMerchant(ctx, userID, input)

	require.NoError(t, err)
	assert.NotNil(t, existingUser.MerchantProfile)
	assert.Equal(t, input.StoreName, existingUser.MerchantProfile.StoreName)
}

func TestProfileService_GetUserRole(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{
		ID:              userID,
		UserProfile:     &entity.UserProfile{},
		MerchantProfile: &entity.MerchantProfile{},
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, roles, 2)
	assert.Contains(t, roles, entity.RoleUser.String())
	assert.Contains(t, roles, entity.RoleMerchant.String())
}

func TestProfileService_UpdateMerchantProfile_Success(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	storeName := "New Store Name"
	storeDescription := "New Description"
	businessLicense := "BL-456"
	input := &usecase.UpdateMerchantProfileInput{
		StoreName:        &storeName,
		StoreDescription: &storeDescription,
		BusinessLicense:  &businessLicense,
	}

	existingUser := &entity.User{
		ID: userID,
		MerchantProfile: &entity.MerchantProfile{
			UserID:           userID,
			StoreName:        "Old Store",
			StoreDescription: "Old Description",
			BusinessLicense:  "BL-123",
		},
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(existingUser, nil)
		mockUserRepo.EXPECT().Update(ctx, mock.AnythingOfType("*entity.User")).Return(nil)
	})

	err := fx.service.UpdateMerchantProfile(ctx, userID, input)

	require.NoError(t, err)
	assert.Equal(t, storeName, existingUser.MerchantProfile.StoreName)
	assert.Equal(t, storeDescription, existingUser.MerchantProfile.StoreDescription)
	assert.Equal(t, businessLicense, existingUser.MerchantProfile.BusinessLicense)
}

func TestProfileService_GetUserRole_OnlyUserProfile(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{
		ID:              userID,
		UserProfile:     &entity.UserProfile{},
		MerchantProfile: nil,
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Contains(t, roles, entity.RoleUser.String())
}

func TestProfileService_GetUserRole_OnlyMerchantProfile(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{
		ID:              userID,
		UserProfile:     nil,
		MerchantProfile: &entity.MerchantProfile{},
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Contains(t, roles, entity.RoleMerchant.String())
}

func TestProfileService_GetUserRole_NoProfiles(t *testing.T) {
	fx := createTestProfileService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{
		ID:              userID,
		UserProfile:     nil,
		MerchantProfile: nil,
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
	})

	roles, err := fx.service.GetUserRole(ctx, userID)

	require.NoError(t, err)
	assert.Empty(t, roles)
}
