package impl

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	mockSvc "radar/internal/mocks/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserService_RegisterUser_Success(t *testing.T) {
	// Setup mocks
	txManager := mockRepo.NewMockTransactionManager(t)
	userRepo := mockRepo.NewMockUserRepository(t)
	authRepo := mockRepo.NewMockAuthRepository(t)
	refreshTokenRepo := mockRepo.NewMockRefreshTokenRepository(t)
	hasher := mockSvc.NewMockPasswordHasher(t)
	tokenService := mockSvc.NewMockTokenService(t)
	googleAuthService := mockSvc.NewMockOAuthAuthService(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewUserService(
		txManager,
		userRepo,
		authRepo,
		refreshTokenRepo,
		hasher,
		tokenService,
		googleAuthService,
		logger,
	)

	ctx := context.Background()
	input := &usecase.RegisterUserInput{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "Password123!",
	}

	// Mock TransactionManager.Execute to run the provided function
	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)

			// Mock find auth not found
			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
				Return(nil, repository.ErrAuthNotFound)

			// Mock hasher.ValidatePasswordStrength
			hasher.EXPECT().ValidatePasswordStrength(input.Password).Return(nil)

			// Mock hasher.Hash
			hasher.EXPECT().Hash(input.Password).Return("hashed_password", nil)

			// Mock user creation
			mockUserRepo.EXPECT().
				Create(ctx, mock.AnythingOfType("*entity.User")).
				Run(func(ctx context.Context, user *entity.User) {
					user.ID = uuid.New()
				}).
				Return(nil)

			// Mock auth creation
			mockAuthRepo.EXPECT().
				CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
				Return(nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	// Execute
	output, err := service.RegisterUser(ctx, input)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, input.Email, output.User.Email)
}

func TestUserService_RegisterUser_InvalidCredentials(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	userRepo := mockRepo.NewMockUserRepository(t)
	authRepo := mockRepo.NewMockAuthRepository(t)
	refreshTokenRepo := mockRepo.NewMockRefreshTokenRepository(t)
	hasher := mockSvc.NewMockPasswordHasher(t)
	tokenService := mockSvc.NewMockTokenService(t)
	googleAuthService := mockSvc.NewMockOAuthAuthService(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewUserService(
		txManager,
		userRepo,
		authRepo,
		refreshTokenRepo,
		hasher,
		tokenService,
		googleAuthService,
		logger,
	)

	ctx := context.Background()
	input := &usecase.RegisterUserInput{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "wrong",
	}

	authRecord := &entity.Authentication{
		UserID:       uuid.New(),
		PasswordHash: "hashed",
		Provider:     entity.ProviderTypeEmail,
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockAuthRepo := mockRepo.NewMockAuthRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().AuthRepo().Return(mockAuthRepo)

			mockAuthRepo.EXPECT().
				FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
				Return(authRecord, nil)

			hasher.EXPECT().Check(input.Password, authRecord.PasswordHash).Return(false)

			_ = fn(mockFactory)
		}).
		Return(assert.AnError)

	output, err := service.RegisterUser(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, output)
}
