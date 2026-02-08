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

func TestSessionService_GetSessionInfo_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()
	token := &entity.RefreshToken{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
	})

	info, err := fx.service.GetSessionInfo(ctx, userID, sessionID)

	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, sessionID, info.ID)
	assert.Equal(t, userID, info.UserID)
	assert.True(t, info.IsActive)
}

func TestSessionService_RevokeAllOtherSessions_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	currentSessionID := uuid.New()
	otherSessionID := uuid.New()
	user := &entity.User{ID: userID}
	tokens := []*entity.RefreshToken{
		{ID: currentSessionID, UserID: userID},
		{ID: otherSessionID, UserID: userID},
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)
		// Only the other session should be deleted
		mockRefreshRepo.EXPECT().DeleteRefreshToken(ctx, otherSessionID).Return(nil)
	})

	err := fx.service.RevokeAllOtherSessions(ctx, userID, currentSessionID)

	require.NoError(t, err)
}

func TestSessionService_CleanupExpiredSessions_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockRefreshRepo.EXPECT().DeleteExpiredRefreshTokens(ctx).Return(nil)
	})

	count, err := fx.service.CleanupExpiredSessions(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSessionService_DetectAnomalousActivity_Success(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()

	// Create tokens that will trigger anomaly detection
	tokens := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
		{ID: uuid.New(), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}, // Rapid creation
	}

	fx.onExecute(ctx, nil, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)
	})

	anomalies, err := fx.service.DetectAnomalousActivity(ctx, userID)

	require.NoError(t, err)
	assert.NotNil(t, anomalies)
	// Should detect rapid session creation
	assert.NotEmpty(t, anomalies)
}

func TestSessionService_GetSessionStatistics_NoSessions(t *testing.T) {
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
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return([]*entity.RefreshToken{}, nil)
	})

	stats, err := fx.service.GetSessionStatistics(ctx, userID)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, 0, stats.TotalActiveSessions)
}

func TestSessionService_GetSessionStatistics_MixedActiveSessions(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()

	tokens := []*entity.RefreshToken{
		{ID: uuid.New(), UserID: userID, CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(time.Hour)},  // Active
		{ID: uuid.New(), UserID: userID, CreatedAt: now.Add(-3 * time.Hour), ExpiresAt: now.Add(-time.Hour)}, // Expired
		{ID: uuid.New(), UserID: userID, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(2 * time.Hour)},  // Active
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
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalSessions)
	assert.Equal(t, 2, stats.TotalActiveSessions)
}
