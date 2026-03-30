package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	mockSvc "radar/internal/mocks/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// userServiceFixtures holds all test dependencies for user service tests.
type userServiceFixtures struct {
	t                 *testing.T
	service           usecase.UserUsecase
	txManager         *mockRepo.MockTransactionManager
	userRepo          *mockRepo.MockUserRepository
	authRepo          *mockRepo.MockAuthRepository
	refreshTokenRepo  *mockRepo.MockRefreshTokenRepository
	hasher            *mockSvc.MockPasswordHasher
	tokenService      *mockSvc.MockTokenService
	googleAuthService *mockSvc.MockOAuthAuthService
}

func createTestUserService(t *testing.T) *userServiceFixtures {
	txManager := mockRepo.NewMockTransactionManager(t)
	userRepo := mockRepo.NewMockUserRepository(t)
	authRepo := mockRepo.NewMockAuthRepository(t)
	refreshTokenRepo := mockRepo.NewMockRefreshTokenRepository(t)
	hasher := mockSvc.NewMockPasswordHasher(t)
	tokenService := mockSvc.NewMockTokenService(t)
	googleAuthService := mockSvc.NewMockOAuthAuthService(t)
	googleAuthService.EXPECT().GetProvider().Return(entity.ProviderTypeGoogle).Maybe()
	logger := newDiscardLogger()

	service := NewUserService(UserServiceParams{
		TxManager:         txManager,
		UserRepo:          userRepo,
		AuthRepo:          authRepo,
		RefreshTokenRepo:  refreshTokenRepo,
		Hasher:            hasher,
		TokenService:      tokenService,
		GoogleAuthService: googleAuthService,
		Config:            newTestConfig(0),
		Logger:            logger,
	})

	return &userServiceFixtures{
		t:                 t,
		service:           service,
		txManager:         txManager,
		userRepo:          userRepo,
		authRepo:          authRepo,
		refreshTokenRepo:  refreshTokenRepo,
		hasher:            hasher,
		tokenService:      tokenService,
		googleAuthService: googleAuthService,
	}
}

// onExecute is a helper method to reduce boilerplate for mocking txManager.Execute.
func (fx *userServiceFixtures) onExecute(ctx context.Context, returnErr error, setupMocks func(factory *mockRepo.MockRepositoryFactory)) {
	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(fx.t)
			setupMocks(mockFactory)
			_ = fn(mockFactory)
		}).
		Return(returnErr)
}

func TestUserService_RegisterUser_Success(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	input := &usecase.RegisterUserInput{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "Password123!",
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
			Return(nil, domainerrors.ErrAuthNotFound)
		mockUserRepo.EXPECT().
			FindByEmail(ctx, input.Email).
			Return(nil, domainerrors.ErrUserNotFound)

		mockUserRepo.EXPECT().
			Create(ctx, mock.AnythingOfType("*entity.User")).
			Run(func(ctx context.Context, user *entity.User) {
				user.ID = uuid.New()
			}).
			Return(nil)

		mockAuthRepo.EXPECT().
			CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
			Return(nil)
	})
	fx.tokenService.EXPECT().
		GenerateTokens(mock.AnythingOfType("uuid.UUID"), []string{"user"}).
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

	output, err := fx.service.RegisterUser(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	assert.Equal(t, input.Email, output.User.Email)
}

func TestUserService_RegisterUser_InvalidCredentials(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.RegisterUserInput{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "wrong",
	}

	fx.hasher.EXPECT().ValidatePasswordStrength(input.Password).Return(nil).Once()
	fx.hasher.EXPECT().Hash(input.Password).Return("hashed_password", nil).Once()

	authRecord := &entity.Authentication{
		UserID:       userID,
		PasswordHash: "hashed",
		Provider:     entity.ProviderTypeEmail,
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockAuthRepo := mockRepo.NewMockAuthRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().AuthRepo().Return(mockAuthRepo)

		mockAuthRepo.EXPECT().
			FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
			Return(authRecord, nil)
		mockUserRepo.EXPECT().
			FindByID(ctx, userID).
			Return(&entity.User{
				ID:          userID,
				Email:       input.Email,
				UserProfile: &entity.UserProfile{UserID: userID},
			}, nil)
	})

	// Password check happens outside the transaction.
	fx.hasher.EXPECT().Check(input.Password, authRecord.PasswordHash).Return(false).Once()

	output, err := fx.service.RegisterUser(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, domainerrors.ErrInvalidCredentials))
}

func TestUserService_Login_InvalidCredentials_DoesNotLoadUser(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.LoginInput{
		Email:    "test@example.com",
		Password: "wrong-password",
	}

	authRecord := &entity.Authentication{
		UserID:         userID,
		Provider:       entity.ProviderTypeEmail,
		ProviderUserID: input.Email,
		PasswordHash:   "hashed-password",
	}

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
				Return(authRecord, nil)
			mockUserRepo.EXPECT().
				FindByID(ctx, userID).
				Return(&entity.User{
					ID:          userID,
					Email:       input.Email,
					UserProfile: &entity.UserProfile{UserID: userID},
				}, nil)

			return fn(mockFactory)
		}).
		Once()

	// Password check happens outside the transaction (bcrypt is CPU-bound).
	fx.hasher.EXPECT().
		Check(input.Password, authRecord.PasswordHash).
		Return(false).
		Once()

	output, err := fx.service.Login(ctx, input)

	require.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, domainerrors.ErrInvalidCredentials))
	fx.txManager.AssertNumberOfCalls(t, "Execute", 1)
	fx.tokenService.AssertNotCalled(t, "GenerateTokens", mock.Anything, mock.Anything)
	fx.refreshTokenRepo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
}

func TestUserService_Login_Success_LoadsUserAfterPasswordCheck(t *testing.T) {
	fx := createTestUserService(t)

	ctx := context.Background()
	input := &usecase.LoginInput{
		Email:    "test@example.com",
		Password: "Password123!",
	}

	userID := uuid.New()
	authRecord := &entity.Authentication{
		UserID:         userID,
		Provider:       entity.ProviderTypeEmail,
		ProviderUserID: input.Email,
		PasswordHash:   "hashed-password",
	}
	user := &entity.User{
		ID:          userID,
		Email:       input.Email,
		Name:        "Test User",
		UserProfile: &entity.UserProfile{UserID: userID},
	}

	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		RunAndReturn(func(ctx context.Context, fn func(repository.RepositoryFactory) error) error {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)

			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)
			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
				Return(authRecord, nil)
			mockUserRepo.EXPECT().
				FindByID(ctx, userID).
				Return(user, nil)

			return fn(mockFactory)
		}).
		Once()

	fx.hasher.EXPECT().
		Check(input.Password, authRecord.PasswordHash).
		Return(true).
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

	fx.refreshTokenRepo.EXPECT().
		CreateRefreshToken(ctx, mock.AnythingOfType("*entity.RefreshToken")).
		Run(func(_ context.Context, token *entity.RefreshToken) {
			assert.Equal(t, userID, token.UserID)
			assert.Equal(t, "refresh-token-hash", token.TokenHash)
			assert.False(t, token.ExpiresAt.IsZero())
		}).
		Return(nil).
		Once()

	output, err := fx.service.Login(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, usecase.AuthStatusAuthenticated, output.Status)
	assert.Equal(t, "access-token", output.AccessToken)
	assert.Equal(t, "refresh-token", output.RefreshToken)
	assert.Equal(t, userID, output.User.ID)
	fx.txManager.AssertNumberOfCalls(t, "Execute", 1)
}
