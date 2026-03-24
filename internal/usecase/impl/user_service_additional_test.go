package impl

import (
	"context"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/errors"
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

	fx.hasher.EXPECT().ValidatePasswordStrength(input.Password).Return(nil).Once()
	fx.hasher.EXPECT().Hash(input.Password).Return("hashed_password", nil).Once()

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockAuthRepo := mockRepo.NewMockAuthRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().AuthRepo().Return(mockAuthRepo)

		mockAuthRepo.EXPECT().
			FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
			Return(nil, repository.ErrAuthNotFound)
		mockUserRepo.EXPECT().
			FindByEmail(ctx, input.Email).
			Return(nil, repository.ErrUserNotFound)

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
	fx.tokenService.EXPECT().
		GenerateTokens(mock.AnythingOfType("uuid.UUID"), []string{"merchant"}).
		Return("access-token", "refresh-token", nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken("refresh-token").
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GetRefreshTokenDuration().
		Return(time.Hour).
		Once()
	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Return(nil).
		Once()

	output, err := fx.service.RegisterMerchant(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.MerchantProfile)
	assert.Equal(t, input.Email, output.User.Email)
	assert.Equal(t, input.StoreName, output.User.MerchantProfile.StoreName)
}

func TestUserService_RegisterUser_ExistingOAuthUserReturnsConflict(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.RegisterUserInput{
		Name:     "OAuth User",
		Email:    "member@example.com",
		Password: "Password123!",
	}

	fx.hasher.EXPECT().ValidatePasswordStrength(input.Password).Return(nil).Once()
	fx.hasher.EXPECT().Hash(input.Password).Return("hashed_password", nil).Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
				Return(nil, repository.ErrAuthNotFound)
			mockUserRepo.EXPECT().
				FindByEmail(ctx, input.Email).
				Return(&entity.User{
					ID:          userID,
					Name:        "Existing OAuth User",
					Email:       input.Email,
					UserProfile: &entity.UserProfile{UserID: userID},
				}, nil)

			return fn(mockFactory)
		}).
		Once()

	output, err := fx.service.RegisterUser(ctx, input)

	require.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, domainerrors.ErrConflict))
	fx.tokenService.AssertNotCalled(t, "GenerateTokens", mock.Anything, mock.Anything)
	fx.refreshTokenRepo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
}

func TestUserService_RegisterMerchant_ExistingOAuthUserReturnsConflict(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.RegisterMerchantInput{
		Name:            "Merchant Owner",
		Email:           "member@example.com",
		Password:        "Password123!",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}

	fx.hasher.EXPECT().ValidatePasswordStrength(input.Password).Return(nil).Once()
	fx.hasher.EXPECT().Hash(input.Password).Return("hashed_password", nil).Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
				Return(nil, repository.ErrAuthNotFound)
			mockUserRepo.EXPECT().
				FindByEmail(ctx, input.Email).
				Return(&entity.User{
					ID:          userID,
					Name:        "Existing OAuth User",
					Email:       input.Email,
					UserProfile: &entity.UserProfile{UserID: userID},
				}, nil)

			return fn(mockFactory)
		}).
		Once()

	output, err := fx.service.RegisterMerchant(ctx, input)

	require.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, domainerrors.ErrConflict))
	fx.tokenService.AssertNotCalled(t, "GenerateTokens", mock.Anything, mock.Anything)
	fx.refreshTokenRepo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
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

func TestUserService_GoogleCallback_ExistingEmailUserLinksGoogleAuth(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.GoogleCallbackInput{
		IDToken: "google-id-token",
		State:   "user",
	}
	oauthUser := &service.OAuthUser{
		ID:            "google-user-id",
		Email:         "member@example.com",
		Name:          "Member User",
		EmailVerified: true,
	}

	fx.googleAuthService.EXPECT().
		VerifyIDToken(ctx, input.IDToken).
		Return(oauthUser, nil).
		Once()
	fx.tokenService.EXPECT().
		GenerateTokens(userID, []string{"user"}).
		Return("access-token", "refresh-token", nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken("refresh-token").
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GetRefreshTokenDuration().
		Return(time.Hour).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)

			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID).
				Return(nil, repository.ErrAuthNotFound)
			mockUserRepo.EXPECT().
				FindByEmail(ctx, oauthUser.Email).
				Return(&entity.User{
					ID:          userID,
					Email:       oauthUser.Email,
					UserProfile: &entity.UserProfile{UserID: userID},
				}, nil)
			mockAuthRepo.EXPECT().
				FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle).
				Return(nil, repository.ErrAuthNotFound)
			mockAuthRepo.EXPECT().
				CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
				Run(func(_ context.Context, auth *entity.Authentication) {
					assert.Equal(t, userID, auth.UserID)
					assert.Equal(t, entity.ProviderTypeGoogle, auth.Provider)
					assert.Equal(t, oauthUser.ID, auth.ProviderUserID)
				}).
				Return(nil)

			return fn(mockFactory)
		}).
		Once()
	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Run(func(_ context.Context, token *entity.RefreshToken) {
			assert.Equal(t, userID, token.UserID)
			assert.Equal(t, "refresh-token-hash", token.TokenHash)
			assert.False(t, token.ExpiresAt.IsZero())
		}).
		Return(nil).
		Once()

	output, err := fx.service.GoogleCallback(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	assert.Equal(t, "access-token", output.AccessToken)
	assert.Equal(t, "refresh-token", output.RefreshToken)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.UserProfile)
}

func TestUserService_GoogleCallback_ExistingEmailUserMerchantStateAttachesMerchantProfile(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.GoogleCallbackInput{
		IDToken:         "google-id-token",
		State:           "merchant",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}
	oauthUser := &service.OAuthUser{
		ID:            "google-user-id",
		Email:         "member@example.com",
		Name:          "Merchant Owner",
		EmailVerified: true,
	}

	fx.googleAuthService.EXPECT().
		VerifyIDToken(ctx, input.IDToken).
		Return(oauthUser, nil).
		Once()
	fx.tokenService.EXPECT().
		GenerateTokens(userID, []string{"user", "merchant"}).
		Return("access-token", "refresh-token", nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken("refresh-token").
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GetRefreshTokenDuration().
		Return(time.Hour).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)

			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID).
				Return(nil, repository.ErrAuthNotFound)
			mockUserRepo.EXPECT().
				FindByEmail(ctx, oauthUser.Email).
				Return(&entity.User{
					ID:          userID,
					Email:       oauthUser.Email,
					UserProfile: &entity.UserProfile{UserID: userID},
				}, nil)
			mockUserRepo.EXPECT().
				Update(ctx, mock.AnythingOfType("*entity.User")).
				Run(func(_ context.Context, user *entity.User) {
					require.NotNil(t, user.MerchantProfile)
					assert.Equal(t, input.StoreName, user.MerchantProfile.StoreName)
					assert.Equal(t, input.BusinessLicense, user.MerchantProfile.BusinessLicense)
				}).
				Return(nil)
			mockAuthRepo.EXPECT().
				FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle).
				Return(nil, repository.ErrAuthNotFound)
			mockAuthRepo.EXPECT().
				CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
				Run(func(_ context.Context, auth *entity.Authentication) {
					assert.Equal(t, userID, auth.UserID)
					assert.Equal(t, entity.ProviderTypeGoogle, auth.Provider)
					assert.Equal(t, oauthUser.ID, auth.ProviderUserID)
				}).
				Return(nil)

			return fn(mockFactory)
		}).
		Once()
	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Return(nil).
		Once()

	output, err := fx.service.GoogleCallback(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.MerchantProfile)
	assert.Equal(t, input.StoreName, output.User.MerchantProfile.StoreName)
}

func TestUserService_GoogleCallback_ExistingGoogleMerchantUserStateAttachesUserProfile(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.GoogleCallbackInput{
		IDToken: "google-id-token",
		State:   "user",
	}
	oauthUser := &service.OAuthUser{
		ID:            "google-user-id",
		Email:         "merchant@example.com",
		Name:          "Merchant Owner",
		EmailVerified: true,
	}

	fx.googleAuthService.EXPECT().
		VerifyIDToken(ctx, input.IDToken).
		Return(oauthUser, nil).
		Once()
	fx.tokenService.EXPECT().
		GenerateTokens(userID, []string{"user", "merchant"}).
		Return("access-token", "refresh-token", nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken("refresh-token").
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GetRefreshTokenDuration().
		Return(time.Hour).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)

			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID).
				Return(&entity.Authentication{UserID: userID}, nil)
			mockUserRepo.EXPECT().
				FindByID(ctx, userID).
				Return(&entity.User{
					ID: userID,
					MerchantProfile: &entity.MerchantProfile{
						UserID:          userID,
						StoreName:       "NomNom Bento",
						BusinessLicense: "A123456789",
					},
				}, nil)
			mockUserRepo.EXPECT().
				Update(ctx, mock.AnythingOfType("*entity.User")).
				Run(func(_ context.Context, user *entity.User) {
					require.NotNil(t, user.UserProfile)
					assert.Equal(t, userID, user.UserProfile.UserID)
				}).
				Return(nil)

			return fn(mockFactory)
		}).
		Once()
	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Run(func(_ context.Context, token *entity.RefreshToken) {
			assert.Equal(t, userID, token.UserID)
			assert.Equal(t, "refresh-token-hash", token.TokenHash)
			assert.False(t, token.ExpiresAt.IsZero())
		}).
		Return(nil).
		Once()

	output, err := fx.service.GoogleCallback(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	assert.Equal(t, "access-token", output.AccessToken)
	assert.Equal(t, "refresh-token", output.RefreshToken)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.UserProfile)
	require.NotNil(t, output.User.MerchantProfile)
}

func TestUserService_GoogleCallback_NewMerchantStateCreatesMerchant(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	input := &usecase.GoogleCallbackInput{
		IDToken:         "google-id-token",
		State:           "merchant",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}
	oauthUser := &service.OAuthUser{
		ID:            "google-user-id",
		Email:         "merchant@example.com",
		Name:          "Merchant Owner",
		EmailVerified: true,
	}

	fx.googleAuthService.EXPECT().
		VerifyIDToken(ctx, input.IDToken).
		Return(oauthUser, nil).
		Once()
	fx.tokenService.EXPECT().
		GenerateTokens(mock.AnythingOfType("uuid.UUID"), []string{"merchant"}).
		Return("access-token", "refresh-token", nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken("refresh-token").
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GetRefreshTokenDuration().
		Return(time.Hour).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)

			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID).
				Return(nil, repository.ErrAuthNotFound)
			mockUserRepo.EXPECT().
				FindByEmail(ctx, oauthUser.Email).
				Return(nil, repository.ErrUserNotFound)
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

			return fn(mockFactory)
		}).
		Once()
	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Return(nil).
		Once()

	output, err := fx.service.GoogleCallback(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.MerchantProfile)
	assert.Equal(t, input.StoreName, output.User.MerchantProfile.StoreName)
}

func TestUserService_GoogleCallback_NewMerchantWithoutDraftReturnsOnboardingRequired(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.GoogleCallbackInput{
		IDToken:       "google-id-token",
		RequestedRole: "merchant",
	}
	oauthUser := &service.OAuthUser{
		ID:            "google-user-id",
		Email:         "merchant@example.com",
		Name:          "Merchant Owner",
		EmailVerified: true,
	}

	fx.googleAuthService.EXPECT().
		VerifyIDToken(ctx, input.IDToken).
		Return(oauthUser, nil).
		Once()
	fx.tokenService.EXPECT().
		GenerateOnboardingToken(userID).
		Return("onboarding-token", nil).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)

			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID).
				Return(nil, repository.ErrAuthNotFound)
			mockUserRepo.EXPECT().
				FindByEmail(ctx, oauthUser.Email).
				Return(nil, repository.ErrUserNotFound)
			mockUserRepo.EXPECT().
				Create(ctx, mock.AnythingOfType("*entity.User")).
				Run(func(_ context.Context, user *entity.User) {
					user.ID = userID
					assert.Nil(t, user.UserProfile)
					assert.Nil(t, user.MerchantProfile)
				}).
				Return(nil)
			mockAuthRepo.EXPECT().
				CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
				Return(nil)

			return fn(mockFactory)
		}).
		Once()

	output, err := fx.service.GoogleCallback(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusOnboardingRequired, output.Status)
	assert.Equal(t, "onboarding-token", output.OnboardingToken)
	assert.Equal(t, entity.RoleMerchant.String(), output.RequestedRole)
	assert.Equal(t, []string{"store_name", "business_license"}, output.RequiredFields)
	assert.Empty(t, output.AccessToken)
	assert.Empty(t, output.RefreshToken)
	assert.Nil(t, output.User)
}

func TestUserService_CompleteMerchantOnboarding_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.CompleteMerchantOnboardingInput{
		OnboardingToken: "onboarding-token",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}

	fx.tokenService.EXPECT().
		ValidateToken(input.OnboardingToken).
		Return(&service.Claims{UserID: userID, Type: "onboarding"}, nil).
		Once()
	fx.tokenService.EXPECT().
		GenerateTokens(userID, []string{"merchant"}).
		Return("access-token", "refresh-token", nil).
		Once()
	fx.tokenService.EXPECT().
		HashToken("refresh-token").
		Return("refresh-token-hash").
		Once()
	fx.tokenService.EXPECT().
		GetRefreshTokenDuration().
		Return(time.Hour).
		Once()
	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Return(nil).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().
				FindByID(ctx, userID).
				Return(&entity.User{
					ID:    userID,
					Name:  "Merchant Owner",
					Email: "merchant@example.com",
				}, nil)
			mockUserRepo.EXPECT().
				Update(ctx, mock.AnythingOfType("*entity.User")).
				Run(func(_ context.Context, user *entity.User) {
					require.NotNil(t, user.MerchantProfile)
					assert.Equal(t, input.StoreName, user.MerchantProfile.StoreName)
					assert.Equal(t, input.BusinessLicense, user.MerchantProfile.BusinessLicense)
				}).
				Return(nil)

			return fn(mockFactory)
		}).
		Once()

	output, err := fx.service.CompleteMerchantOnboarding(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	assert.Equal(t, "access-token", output.AccessToken)
	assert.Equal(t, "refresh-token", output.RefreshToken)
	require.NotNil(t, output.User)
	require.NotNil(t, output.User.MerchantProfile)
	assert.Equal(t, input.StoreName, output.User.MerchantProfile.StoreName)
}

func TestUserService_CompleteMerchantOnboarding_AlreadyCompletedReturnsConflict(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.CompleteMerchantOnboardingInput{
		OnboardingToken: "onboarding-token",
		StoreName:       "NomNom Bento",
		BusinessLicense: "A123456789",
	}

	fx.tokenService.EXPECT().
		ValidateToken(input.OnboardingToken).
		Return(&service.Claims{UserID: userID, Type: "onboarding"}, nil).
		Once()

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockUserRepo.EXPECT().
				FindByID(ctx, userID).
				Return(&entity.User{
					ID:    userID,
					Name:  "Merchant Owner",
					Email: "merchant@example.com",
					MerchantProfile: &entity.MerchantProfile{
						UserID:          userID,
						StoreName:       "Existing Store",
						BusinessLicense: "B987654321",
					},
				}, nil)

			return fn(mockFactory)
		}).
		Once()

	output, err := fx.service.CompleteMerchantOnboarding(ctx, input)

	require.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, domainerrors.ErrConflict))
	fx.tokenService.AssertNotCalled(t, "GenerateTokens", mock.Anything, mock.Anything)
	fx.refreshTokenRepo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
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

	merchantUser, err := buildNewMerchantEntity(input)
	require.NoError(t, err)
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
