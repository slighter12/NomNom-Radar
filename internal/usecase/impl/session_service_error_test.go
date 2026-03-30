package impl

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	mockRepo "radar/internal/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSessionService_CleanupExpiredSessions_Error(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	dbError := errors.New("database connection failed")

	fx.onExecute(ctx, fmt.Errorf("failed to delete expired sessions: %w", dbError), func(factory *mockRepo.MockRepositoryFactory) {
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

	fx.onExecute(ctx, fmt.Errorf("session does not belong to user: %w", domainerrors.ErrForbidden), func(factory *mockRepo.MockRepositoryFactory) {
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

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, expectedErr)
	})

	sessions, err := fx.service.GetActiveSessions(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, sessions)
	assert.ErrorIs(t, err, expectedErr)

	// Test FindRefreshTokensByUserID error
	fx2 := createTestSessionService(t)
	expectedTokensErr := errors.New("db error")
	fx2.onExecute(ctx, expectedTokensErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, expectedTokensErr)
	})

	sessions, err = fx2.service.GetActiveSessions(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, sessions)
	assert.ErrorIs(t, err, expectedTokensErr)
}

func TestSessionService_RevokeSession_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(nil, domainerrors.ErrRefreshTokenNotFound)
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

	fx.onExecute(ctx, fmt.Errorf("session does not belong to user: %w", domainerrors.ErrForbidden), func(factory *mockRepo.MockRepositoryFactory) {
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

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(token, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshToken(ctx, sessionID).Return(expectedErr)
	})

	err := fx.service.RevokeSession(ctx, userID, sessionID)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestSessionService_RevokeAllSessions_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshTokensByUserID(ctx, userID).Return(expectedErr)
	})

	err := fx.service.RevokeAllSessions(ctx, userID)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestSessionService_RevokeAllOtherSessions_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	currentSessionID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, expectedErr)
	})

	err := fx.service.RevokeAllOtherSessions(ctx, userID, currentSessionID)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestSessionService_RevokeAllOtherSessions_DeleteError(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	currentSessionID := uuid.New()
	otherSessionID := uuid.New()
	user := &entity.User{ID: userID}
	dbErr := errors.New("db error")
	tokens := []*entity.RefreshToken{
		{ID: currentSessionID, UserID: userID},
		{ID: otherSessionID, UserID: userID},
	}

	fx.onExecute(ctx, dbErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(tokens, nil)
		mockRefreshRepo.EXPECT().DeleteRefreshToken(ctx, otherSessionID).Return(dbErr)
	})

	err := fx.service.RevokeAllOtherSessions(ctx, userID, currentSessionID)

	assert.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
}

func TestSessionService_GetSessionInfo_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokenByID(ctx, sessionID).Return(nil, domainerrors.ErrRefreshTokenNotFound)
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

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, expectedErr)
	})

	anomalies, err := fx.service.DetectAnomalousActivity(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, anomalies)
	assert.ErrorIs(t, err, expectedErr)
}

func TestSessionService_GetSessionStatistics_UserNotFound(t *testing.T) {
	fx := createTestSessionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.onExecute(ctx, domainerrors.ErrNotFound, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)
		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)
		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(nil, domainerrors.ErrUserNotFound)
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

	expectedErr := errors.New("db error")
	fx.onExecute(ctx, expectedErr, func(factory *mockRepo.MockRepositoryFactory) {
		mockUserRepo := mockRepo.NewMockUserRepository(t)
		mockRefreshRepo := mockRepo.NewMockRefreshTokenRepository(t)

		factory.EXPECT().UserRepo().Return(mockUserRepo)
		factory.EXPECT().RefreshTokenRepo().Return(mockRefreshRepo)

		mockUserRepo.EXPECT().FindByID(ctx, userID).Return(user, nil)
		mockRefreshRepo.EXPECT().FindRefreshTokensByUserID(ctx, userID).Return(nil, expectedErr)
	})

	stats, err := fx.service.GetSessionStatistics(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.ErrorIs(t, err, expectedErr)
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
