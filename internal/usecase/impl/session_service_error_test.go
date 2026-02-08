package impl

import (
	"context"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

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

func TestSessionService_GetActiveSessions_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	sessions, err := fx.service.GetActiveSessions(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, sessions)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_GetActiveSessions_FindError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, errors.New("db error"))
	})

	sessions, err := fx.service.GetActiveSessions(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, sessions)

	// Test FindRefreshTokensByUserID error
	fx2 := createTestSessionService(t)
	fx2.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find refresh tokens"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, errors.New("db error"))
	})

	sessions, err = fx2.service.GetActiveSessions(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, sessions)
}

func TestSessionService_RevokeSession_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	err := fx.service.RevokeSession(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_RevokeSession_SessionNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "session not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(nil, repository.ErrRefreshTokenNotFound)
	})

	err := fx.service.RevokeSession(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_RevokeSession_Forbidden(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	token := &entity.RefreshToken{ID: sessionID, UserID: otherUserID}

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrForbidden, "session does not belong to user"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
	})

	err := fx.service.RevokeSession(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrForbidden))
}

func TestSessionService_RevokeSession_DeleteError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}
	token := &entity.RefreshToken{ID: sessionID, UserID: userID}

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to delete session"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshToken(ctx, sessionID).Return(errors.New("db error"))
	})

	err := fx.service.RevokeSession(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete session")
}

func TestSessionService_RevokeAllSessions_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	err := fx.service.RevokeAllSessions(ctx, userID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_RevokeAllSessions_DeleteError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to delete all sessions"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshTokensByUserID(ctx, userID).Return(errors.New("db error"))
	})

	err := fx.service.RevokeAllSessions(ctx, userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete all sessions")
}

func TestSessionService_RevokeAllOtherSessions_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	currentSessionID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	err := fx.service.RevokeAllOtherSessions(ctx, userID, currentSessionID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_RevokeAllOtherSessions_FindTokensError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	currentSessionID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find refresh tokens"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, errors.New("db error"))
	})

	err := fx.service.RevokeAllOtherSessions(ctx, userID, currentSessionID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find refresh tokens")
}

func TestSessionService_GetSessionInfo_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	info, err := fx.service.GetSessionInfo(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.Nil(t, info)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_GetSessionInfo_SessionNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "session not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(nil, repository.ErrRefreshTokenNotFound)
	})

	info, err := fx.service.GetSessionInfo(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.Nil(t, info)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_DetectAnomalousActivity_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	anomalies, err := fx.service.DetectAnomalousActivity(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, anomalies)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_DetectAnomalousActivity_FindTokensError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find refresh tokens"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, errors.New("db error"))
	})

	anomalies, err := fx.service.DetectAnomalousActivity(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, anomalies)
}

func TestSessionService_GetSessionStatistics_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, errors.Wrap(domainerrors.ErrNotFound, "user not found"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, repository.ErrUserNotFound)
	})

	stats, err := fx.service.GetSessionStatistics(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.True(t, errors.Is(err, domainerrors.ErrNotFound))
}

func TestSessionService_GetSessionStatistics_FindTokensError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}

	fx.onExecute(ctx, errors.Wrap(errors.New("db error"), "failed to find refresh tokens"), func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, errors.New("db error"))
	})

	stats, err := fx.service.GetSessionStatistics(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, stats)
}

func TestSessionService_DetectAnomalousActivity_ExcessiveSessions(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()

	// Create more than 10 active sessions to trigger excessive sessions anomaly
	tokens := make([]*entity.RefreshToken, 15)
	for i := range tokens {
		tokens[i] = &entity.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
			ExpiresAt: now.Add(time.Hour),
		}
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

	assert.NoError(t, err)
	assert.NotNil(t, anomalies)
	// Should detect excessive sessions
	hasExcessiveAnomaly := false
	for _, a := range anomalies {
		if a.Type == "excessive_sessions" {
			hasExcessiveAnomaly = true

			break
		}
	}
	assert.True(t, hasExcessiveAnomaly)
}

func TestSessionService_DetectAnomalousActivity_LongLivedSession(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	user := &entity.User{ID: userID}
	now := time.Now()

	// Create a session that's been active for more than 30 days
	tokens := []*entity.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			CreatedAt: now.Add(-35 * 24 * time.Hour), // 35 days ago
			ExpiresAt: now.Add(time.Hour),            // Still active
		},
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

	assert.NoError(t, err)
	assert.NotNil(t, anomalies)
	// Should detect long-lived session
	hasLongLivedAnomaly := false
	for _, a := range anomalies {
		if a.Type == "long_lived_session" {
			hasLongLivedAnomaly = true

			break
		}
	}
	assert.True(t, hasLongLivedAnomaly)
}
