package impl

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSessionService_GetActiveSessions_Success(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	tokens := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
			mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	sessions, err := service.GetActiveSessions(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, tokens[0].ID, sessions[0].ID)
}

func TestSessionService_RevokeSession_Success(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	token := &entity.RefreshToken{ID: sessionID, UserID: userID}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
			mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
			mockRefreshRepo.EXPECT().DeleteRefreshToken(ctx, sessionID).Return(nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	err := service.RevokeSession(ctx, userID, sessionID)

	require.NoError(t, err)
}

func TestSessionService_RevokeAllSessions_Success(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
			mockRefreshRepo.EXPECT().DeleteRefreshTokensByUserID(ctx, userID).Return(nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	err := service.RevokeAllSessions(ctx, userID)

	require.NoError(t, err)
}

func TestSessionService_GetSessionStatistics(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()
	tokens := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)},
		{ID: uuid.New(), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)},
	}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
			mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)

			_ = fn(mockFactory)
		}).
		Return(nil)

	stats, err := service.GetSessionStatistics(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalSessions)
	assert.Equal(t, 2, stats.TotalActiveSessions)
	assert.NotNil(t, stats.OldestSession)
	assert.NotNil(t, stats.NewestSession)
}

func TestSessionService_CleanupExpiredSessions_Error(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger)

	ctx := context.Background()

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
			mockRefreshRepo.EXPECT().DeleteExpiredRefreshTokens(ctx).Return(assert.AnError)

			_ = fn(mockFactory)
		}).
		Return(assert.AnError)

	_, err := service.CleanupExpiredSessions(ctx)

	assert.Error(t, err)
}

func TestSessionService_GetSessionInfo_OwnerMismatch(t *testing.T) {
	txManager := mockRepo.NewMockTransactionManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewSessionService(txManager, logger)

	ctx := context.Background()
	userID := uuid.New()
	tokenOwner := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	token := &entity.RefreshToken{ID: sessionID, UserID: tokenOwner}

	txManager.EXPECT().
		Execute(ctx, mock.AnythingOfType("func(repository.RepositoryFactory) error")).
		Run(func(ctx context.Context, fn func(repository.RepositoryFactory) error) {
			mockFactory := mockRepo.NewMockRepositoryFactory(t)
			mockUserRepo := mockRepo.NewMockUserRepository(t)
			mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

			mockFactory.EXPECT().UserRepo().Return(mockUserRepo)
			mockFactory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

			mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
			mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)

			_ = fn(mockFactory)
		}).
		Return(assert.AnError)

	info, err := service.GetSessionInfo(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.Nil(t, info)
}
