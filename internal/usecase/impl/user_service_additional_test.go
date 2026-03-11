package impl

import (
	"context"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserService_RegisterMerchant_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	input := &usecase.RegisterMerchantInput{
		Name:            "Merchant Owner",
		Email:           "merchant@example.com",
		Password:        "Password123!",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockAuthRepo := mockRepo.NewMockAuthRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().AuthRepo().Return(mockAuthRepo)

		mockAuthRepo.EXPECT().
			FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
			Return(nil, repository.ErrAuthNotFound)

		fx.hasher.EXPECT().ValidatePasswordStrength(input.Password).Return(nil)
		fx.hasher.EXPECT().Hash(input.Password).Return("hashed_password", nil)

		mockUserRepo.EXPECT().
			Create(ctx, mock.AnythingOfType("*entity.User")).
			Run(func(_ context.Context, user *entity.User) {
				user.ID = uuid.New()
				require.NotNil(t, user.MerchantProfile)
				assert.Equal(t, input.StoreName, user.MerchantProfile.StoreName)
				assert.Equal(t, input.BusinessLicense, user.MerchantProfile.BusinessLicense)
			}).
			Return(nil)

		mockAuthRepo.EXPECT().
			CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
			Return(nil)
	})

	output, err := fx.service.RegisterMerchant(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.MerchantProfile)
	assert.Equal(t, input.Email, output.User.Email)
	assert.Equal(t, input.StoreName, output.User.MerchantProfile.StoreName)
}

func TestUserService_RefreshToken_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.RefreshTokenInput{RefreshToken: "refresh-token"}

	fx.tokenService.EXPECT().
		ValidateToken(input.RefreshToken).
		Return(&service.Claims{UserID: userID}, nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken(input.RefreshToken).
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GenerateTokens(userID, []string{"user", "merchant"}).
		Return("new-access-token", "ignored-refresh-token", nil).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

			mockRefreshRepo.EXPECT().
				FindRefreshTokenByHash(ctx, "refresh-token-hash").
				Return(&entity.RefreshToken{ID: uuid.New(), UserID: userID}, nil)
			mockUserRepo.EXPECT().
				FindByID(ctx, userID).
				Return(&entity.User{
					ID:              userID,
					UserProfile:     &entity.UserProfile{UserID: userID},
					MerchantProfile: &entity.MerchantProfile{UserID: userID},
				}, nil)

			require.NoError(t, fn(mockFactory))
		}).
		Return(nil).
		Once()

	output, err := fx.service.RefreshToken(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "new-access-token", output.AccessToken)
}

func TestUserService_Logout_InvalidTokenStillDeletesRefreshToken(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	input := &usecase.LogoutInput{RefreshToken: "stale-refresh-token"}

	fx.tokenService.EXPECT().
		ValidateToken(input.RefreshToken).
		Return(nil, assert.AnError).
		Once()
	fx.tokenService.EXPECT().
		HashToken(input.RefreshToken).
		Return("stale-refresh-token-hash").
		Once()
	fx.refreshTokenRepo.EXPECT().
		DeleteRefreshTokenByHash(ctx, "stale-refresh-token-hash").
		Return(nil).
		Once()

	require.NoError(t, fx.service.Logout(ctx, input))
}

func TestUserService_LogoutAllDevices_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.refreshTokenRepo.EXPECT().
		DeleteRefreshTokensByUserID(ctx, userID).
		Return(nil).
		Once()

	require.NoError(t, fx.service.LogoutAllDevices(ctx, userID))
}

func TestUserService_GetActiveSessions_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessions := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID},
	}

	fx.refreshTokenRepo.EXPECT().
		FindRefreshTokensByUserID(ctx, userID).
		Return(sessions, nil).
		Once()

	got, err := fx.service.GetActiveSessions(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, sessions, got)
}

func TestUserService_RevokeSession_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	tokenID := uuid.New()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
			mockRefreshRepo.EXPECT().
				FindRefreshTokenByID(ctx, tokenID).
				Return(&entity.RefreshToken{ID: tokenID, UserID: userID}, nil)
			mockRefreshRepo.EXPECT().
				DeleteRefreshToken(ctx, tokenID).
				Return(nil)

			require.NoError(t, fn(mockFactory))
		}).
		Return(nil).
		Once()

	require.NoError(t, fx.service.RevokeSession(ctx, userID, tokenID))
}

func TestUserService_HelperFunctions(t *testing.T) {
	input := &usecase.RegisterMerchantInput{
		Name:            "Merchant Owner",
		Email:           "merchant@example.com",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}

	merchantUser := buildNewMerchantEntity(input)
	require.NotNil(t, merchantUser.MerchantProfile)
	assert.Equal(t, input.Name, merchantUser.Name)
	assert.Equal(t, input.Email, merchantUser.Email)
	assert.Equal(t, input.StoreName, merchantUser.MerchantProfile.StoreName)
	assert.Equal(t, input.BusinessLicense, merchantUser.MerchantProfile.BusinessLicense)

	user := &entity.User{ID: uuid.New()}
	assert.False(t, userHasUserProfile(user))
	assert.False(t, userHasMerchantProfile(user))

	attachUserProfile(user)
	require.NotNil(t, user.UserProfile)
	assert.Equal(t, user.ID, user.UserProfile.UserID)
	assert.True(t, userHasUserProfile(user))

	attachMerchantProfile(input)(user)
	require.NotNil(t, user.MerchantProfile)
	assert.Equal(t, user.ID, user.MerchantProfile.UserID)
	assert.Equal(t, input.StoreName, user.MerchantProfile.StoreName)
	assert.True(t, userHasMerchantProfile(user))

	assert.Equal(t, "***", maskEmailForLog("invalid"))
	assert.Equal(t, "***@ex.com", maskEmailForLog("ab@ex.com"))
	assert.Equal(t, "lon***@example.com", maskEmailForLog("longer@example.com"))
}

func TestUserService_RevokeSession_Forbidden(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	tokenID := uuid.New()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
			mockRefreshRepo.EXPECT().
				FindRefreshTokenByID(ctx, tokenID).
				Return(&entity.RefreshToken{ID: tokenID, UserID: uuid.New()}, nil)

			return fn(mockFactory)
		}).
		Once()

	err := fx.service.RevokeSession(ctx, userID, tokenID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrForbidden)
}
