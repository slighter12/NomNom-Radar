package impl

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// sessionServiceFixtures holds all test dependencies for session service tests.
type sessionServiceFixtures struct {
	t         *testing.T
	service   *sessionService
	txManager *mockRepo.MockTransactionManager
}

func createTestSessionService(t *testing.T) *sessionServiceFixtures {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger).(*sessionService)

	return &sessionServiceFixtures{
		t:         t,
		service:   service,
		txManager: txManager,
	}
}

// onExecute is a helper method to reduce boilerplate for mocking txManager.Execute.
func (fx *sessionServiceFixtures) onExecute(ctx context.Context, returnErr error, setupMocks func(factory *mockRepo.MockRepositoryFactory)) {
	fx.txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(fx.t)
			setupMocks(mockFactory)
			_ = fn(mockFactory)
		}).
		Return(returnErr)
}

func TestSessionService_GetActiveSessions_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	tokens := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)
	})

	sessions, err := fx.service.GetActiveSessions(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, tokens[0].ID, sessions[0].ID)
}

func TestSessionService_RevokeSession_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	token := &entity.RefreshToken{ID: sessionID, UserID: userID}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshToken(ctx, sessionID).Return(nil)
	})

	err := fx.service.RevokeSession(ctx, userID, sessionID)

	require.NoError(t, err)
}

func TestSessionService_RevokeAllSessions_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshTokensByUserID(ctx, userID).Return(nil)
	})

	err := fx.service.RevokeAllSessions(ctx, userID)

	require.NoError(t, err)
}

func TestSessionService_GetSessionStatistics(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()
	tokens := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)},
		{ID: uuid.New(), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)},
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)
	})

	stats, err := fx.service.GetSessionStatistics(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalSessions)
	assert.Equal(t, 2, stats.TotalActiveSessions)
	assert.NotNil(t, stats.OldestSession)
	assert.NotNil(t, stats.NewestSession)
}

func TestSessionService_CleanupExpiredSessions_Error(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	dbError := errors.New("database connection failed")

	fx.onExecute(ctx, errors.Wrap(dbError, "failed to delete expired sessions"), func(factory *mockRepo.MockRepositoryFactory) {
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockRefreshRepo.EXPECT().DeleteExpiredRefreshTokens(ctx).Return(dbError)
	})

	_, err := fx.service.CleanupExpiredSessions(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup expired sessions")
}

func TestSessionService_GetSessionInfo_OwnerMismatch(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	tokenOwner := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	token := &entity.RefreshToken{ID: sessionID, UserID: tokenOwner}

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrForbidden, "session does not belong to user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
	})

	info, err := fx.service.GetSessionInfo(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.Nil(t, info)
	assert.True(t, errors.Is(err, domainerrors.ErrForbidden))
}
