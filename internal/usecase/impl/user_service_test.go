package impl

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	mockSvc "radar/internal/mocks/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewUserService(UserServiceParams{
		TxManager:         txManager,
		UserRepo:          userRepo,
		AuthRepo:          authRepo,
		RefreshTokenRepo:  refreshTokenRepo,
		Hasher:            hasher,
		TokenService:      tokenService,
		GoogleAuthService: googleAuthService,
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
			Run(func(ctx context.Context, user *entity.User) {
				user.ID = uuid.New()
			}).
			Return(nil)

		mockAuthRepo.EXPECT().
			CreateAuthentication(ctx, mock.AnythingOfType("*entity.Authentication")).
			Return(nil)
	})

	output, err := fx.service.RegisterUser(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, input.Email, output.User.Email)
}

func TestUserService_RegisterUser_InvalidCredentials(t *testing.T) {
	fx := createTestUserService(t)

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

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrInvalidCredentials, "password mismatch during registration"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockAuthRepo := mockRepo.NewMockAuthRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().AuthRepo().Return(mockAuthRepo)

		mockAuthRepo.EXPECT().
			FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email).
			Return(authRecord, nil)

		fx.hasher.EXPECT().Check(input.Password, authRecord.PasswordHash).Return(false)
	})

	output, err := fx.service.RegisterUser(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, domainerrors.ErrInvalidCredentials))
}
