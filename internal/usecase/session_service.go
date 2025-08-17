// Package usecase contains the application-specific business rules.
package usecase

import (
	"context"
	"log/slog"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// sessionService implements the SessionUsecase interface.
type sessionService struct {
	fx.In

	txManager repository.TransactionManager
	logger    *slog.Logger
}

// NewSessionService is the constructor for sessionService.
func NewSessionService(
	txManager repository.TransactionManager,
	logger *slog.Logger,
) SessionUsecase {
	return &sessionService{
		txManager: txManager,
		logger:    logger,
	}
}

// GetActiveSessions retrieves all active sessions for a user.
func (srv *sessionService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.SessionInfo, error) {
	srv.logger.Debug("Getting active sessions", "userID", userID)

	var sessions []*entity.SessionInfo

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Get all refresh tokens for the user
		tokens, err := refreshRepo.FindRefreshTokensByUserID(ctx, userID)
		if err != nil {
			return errors.Wrap(err, "failed to find refresh tokens")
		}

		// 3. Convert to session info
		now := time.Now()
		for _, token := range tokens {
			sessions = append(sessions, &entity.SessionInfo{
				ID:        token.ID,
				UserID:    token.UserID,
				CreatedAt: token.CreatedAt,
				ExpiresAt: token.ExpiresAt,
				IsActive:  token.ExpiresAt.After(now),
				LastUsed:  &token.CreatedAt, // In a real implementation, you'd track last usage
			})
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to get active sessions", "error", err, "userID", userID)

		return nil, errors.Wrap(err, "failed to get active sessions")
	}
	srv.logger.Debug("Successfully retrieved active sessions", "userID", userID, "count", len(sessions))

	return sessions, nil
}

// RevokeSession revokes a specific session.
func (srv *sessionService) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	srv.logger.Info("Revoking session", "userID", userID, "sessionID", sessionID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Find the session
		token, err := refreshRepo.FindRefreshTokenByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, repository.ErrRefreshTokenNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "session not found")
			}

			return errors.Wrap(err, "failed to find session")
		}

		// 3. Verify ownership
		if token.UserID != userID {
			return errors.Wrap(domainerrors.ErrForbidden, "session does not belong to user")
		}

		// 4. Delete the session
		if err := refreshRepo.DeleteRefreshToken(ctx, sessionID); err != nil {
			return errors.Wrap(err, "failed to delete session")
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to revoke session", "error", err, "userID", userID, "sessionID", sessionID)

		return errors.Wrap(err, "failed to revoke session")
	}
	srv.logger.Info("Successfully revoked session", "userID", userID, "sessionID", sessionID)

	return nil
}

// RevokeAllSessions revokes all sessions for a user.
func (srv *sessionService) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	srv.logger.Info("Revoking all sessions", "userID", userID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Delete all sessions
		if err := refreshRepo.DeleteRefreshTokensByUserID(ctx, userID); err != nil {
			return errors.Wrap(err, "failed to delete all sessions")
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to revoke all sessions", "error", err, "userID", userID)

		return errors.Wrap(err, "failed to revoke all sessions")
	}
	srv.logger.Info("Successfully revoked all sessions", "userID", userID)

	return nil
}

// RevokeAllOtherSessions revokes all sessions except the current one.
func (srv *sessionService) RevokeAllOtherSessions(ctx context.Context, userID uuid.UUID, currentSessionID uuid.UUID) error {
	srv.logger.Info("Revoking all other sessions", "userID", userID, "currentSessionID", currentSessionID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Get all sessions
		tokens, err := refreshRepo.FindRefreshTokensByUserID(ctx, userID)
		if err != nil {
			return errors.Wrap(err, "failed to find refresh tokens")
		}

		// 3. Delete all sessions except the current one
		for _, token := range tokens {
			if token.ID != currentSessionID {
				if err := refreshRepo.DeleteRefreshToken(ctx, token.ID); err != nil {
					srv.logger.Warn("Failed to delete session", "sessionID", token.ID, "error", err)
					// Continue with other sessions
				}
			}
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to revoke all other sessions", "error", err, "userID", userID)

		return errors.Wrap(err, "failed to revoke all other sessions")
	}
	srv.logger.Info("Successfully revoked all other sessions", "userID", userID, "currentSessionID", currentSessionID)

	return nil
}

// GetSessionInfo retrieves detailed information about a specific session.
func (srv *sessionService) GetSessionInfo(ctx context.Context, userID, sessionID uuid.UUID) (*entity.SessionInfo, error) {
	srv.logger.Debug("Getting session info", "userID", userID, "sessionID", sessionID)

	var sessionInfo *entity.SessionInfo

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Find the session
		token, err := refreshRepo.FindRefreshTokenByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, repository.ErrRefreshTokenNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "session not found")
			}

			return errors.Wrap(err, "failed to find session")
		}

		// 3. Verify ownership
		if token.UserID != userID {
			return errors.Wrap(domainerrors.ErrForbidden, "session does not belong to user")
		}

		// 4. Create session info
		now := time.Now()
		sessionInfo = &entity.SessionInfo{
			ID:        token.ID,
			UserID:    token.UserID,
			CreatedAt: token.CreatedAt,
			ExpiresAt: token.ExpiresAt,
			IsActive:  token.ExpiresAt.After(now),
			LastUsed:  &token.CreatedAt,
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to get session info", "error", err, "userID", userID, "sessionID", sessionID)

		return nil, errors.Wrap(err, "failed to get session info")
	}
	srv.logger.Debug("Successfully retrieved session info", "userID", userID, "sessionID", sessionID)

	return sessionInfo, nil
}

// CleanupExpiredSessions removes all expired sessions from the database.
func (srv *sessionService) CleanupExpiredSessions(ctx context.Context) (int, error) {
	srv.logger.Info("Cleaning up expired sessions")

	var deletedCount int

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// Delete expired sessions
		if err := refreshRepo.DeleteExpiredRefreshTokens(ctx); err != nil {
			return errors.Wrap(err, "failed to delete expired sessions")
		}

		// Note: In a real implementation, you might want to return the count
		// from the repository method
		deletedCount = 0 // Placeholder

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to cleanup expired sessions", "error", err)

		return 0, errors.Wrap(err, "failed to cleanup expired sessions")
	}
	srv.logger.Info("Successfully cleaned up expired sessions", "deletedCount", deletedCount)

	return deletedCount, nil
}

// DetectAnomalousActivity analyzes user sessions for suspicious patterns.
func (srv *sessionService) DetectAnomalousActivity(ctx context.Context, userID uuid.UUID) ([]*entity.AnomalousActivity, error) {
	srv.logger.Debug("Detecting anomalous activity", "userID", userID)

	var anomalies []*entity.AnomalousActivity

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Get user sessions
		tokens, err := refreshRepo.FindRefreshTokensByUserID(ctx, userID)
		if err != nil {
			return errors.Wrap(err, "failed to find refresh tokens")
		}

		// 3. Analyze for anomalies
		now := time.Now()
		anomalies = srv.detectSessionAnomalies(tokens, now)

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to detect anomalous activity", "error", err, "userID", userID)

		return nil, errors.Wrap(err, "failed to detect anomalous activity")
	}
	srv.logger.Debug("Successfully analyzed for anomalous activity", "userID", userID, "anomaliesFound", len(anomalies))

	return anomalies, nil
}

// detectSessionAnomalies analyzes refresh tokens for various anomaly patterns
func (srv *sessionService) detectSessionAnomalies(tokens []*entity.RefreshToken, now time.Time) []*entity.AnomalousActivity {
	var anomalies []*entity.AnomalousActivity

	// Check for excessive active sessions
	if excessiveAnomaly := srv.detectExcessiveSessions(tokens, now); excessiveAnomaly != nil {
		anomalies = append(anomalies, excessiveAnomaly)
	}

	// Check for rapid session creation
	rapidAnomalies := srv.detectRapidSessionCreation(tokens, now)
	anomalies = append(anomalies, rapidAnomalies...)

	// Check for long-lived sessions
	longLivedAnomalies := srv.detectLongLivedSessions(tokens, now)
	anomalies = append(anomalies, longLivedAnomalies...)

	return anomalies
}

// detectExcessiveSessions checks if user has too many active sessions
func (srv *sessionService) detectExcessiveSessions(tokens []*entity.RefreshToken, now time.Time) *entity.AnomalousActivity {
	activeCount := 0
	for _, token := range tokens {
		if token.ExpiresAt.After(now) {
			activeCount++
		}
	}

	if activeCount > 10 { // Configurable threshold
		return &entity.AnomalousActivity{
			Type:        "excessive_sessions",
			Description: "User has an unusually high number of active sessions",
			Severity:    "medium",
			DetectedAt:  now,
		}
	}

	return nil
}

// detectRapidSessionCreation checks for sessions created in rapid succession
func (srv *sessionService) detectRapidSessionCreation(tokens []*entity.RefreshToken, now time.Time) []*entity.AnomalousActivity {
	var anomalies []*entity.AnomalousActivity

	if len(tokens) < 2 {
		return anomalies
	}

	for i := 1; i < len(tokens); i++ {
		timeDiff := tokens[i].CreatedAt.Sub(tokens[i-1].CreatedAt)
		if timeDiff < 5*time.Minute { // Sessions created within 5 minutes
			sessionID := tokens[i].ID
			anomalies = append(anomalies, &entity.AnomalousActivity{
				Type:        "rapid_session_creation",
				Description: "Multiple sessions created in rapid succession",
				Severity:    "high",
				DetectedAt:  now,
				SessionID:   &sessionID,
			})
		}
	}

	return anomalies
}

// detectLongLivedSessions checks for very old sessions (potential forgotten devices)
func (srv *sessionService) detectLongLivedSessions(tokens []*entity.RefreshToken, now time.Time) []*entity.AnomalousActivity {
	var anomalies []*entity.AnomalousActivity

	for _, token := range tokens {
		if token.ExpiresAt.After(now) && now.Sub(token.CreatedAt) > 30*24*time.Hour { // 30 days
			sessionID := token.ID
			anomalies = append(anomalies, &entity.AnomalousActivity{
				Type:        "long_lived_session",
				Description: "Session has been active for an unusually long time",
				Severity:    "low",
				DetectedAt:  now,
				SessionID:   &sessionID,
			})
		}
	}

	return anomalies
}

// GetSessionStatistics provides statistical overview of user's session activity.
func (srv *sessionService) GetSessionStatistics(ctx context.Context, userID uuid.UUID) (*entity.SessionStatistics, error) {
	srv.logger.Debug("Getting session statistics", "userID", userID)

	var stats *entity.SessionStatistics

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Get user sessions
		tokens, err := refreshRepo.FindRefreshTokensByUserID(ctx, userID)
		if err != nil {
			return errors.Wrap(err, "failed to find refresh tokens")
		}

		// 3. Calculate statistics
		now := time.Now()
		stats = srv.calculateSessionStatistics(tokens, now)

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to get session statistics", "error", err, "userID", userID)

		return nil, errors.Wrap(err, "failed to get session statistics")
	}
	srv.logger.Debug("Successfully retrieved session statistics", "userID", userID)

	return stats, nil
}

// calculateSessionStatistics calculates various session statistics from refresh tokens
func (srv *sessionService) calculateSessionStatistics(tokens []*entity.RefreshToken, now time.Time) *entity.SessionStatistics {
	if len(tokens) == 0 {
		return &entity.SessionStatistics{
			TotalActiveSessions: 0,
			TotalSessions:       0,
		}
	}

	activeCount := srv.countActiveSessions(tokens, now)
	oldest, newest := srv.findSessionTimeBounds(tokens)

	stats := &entity.SessionStatistics{
		TotalActiveSessions: activeCount,
		TotalSessions:       len(tokens),
		OldestSession:       &oldest,
		NewestSession:       &newest,
	}

	return stats
}

// countActiveSessions counts the number of currently active sessions
func (srv *sessionService) countActiveSessions(tokens []*entity.RefreshToken, now time.Time) int {
	activeCount := 0
	for _, token := range tokens {
		if token.ExpiresAt.After(now) {
			activeCount++
		}
	}
	srv.logger.Debug("active count", "activeCount", activeCount)

	return activeCount
}

// findSessionTimeBounds finds the oldest and newest session creation times
func (srv *sessionService) findSessionTimeBounds(tokens []*entity.RefreshToken) (oldest, newest time.Time) {
	if len(tokens) == 0 {
		return oldest, newest
	}

	oldest = tokens[0].CreatedAt
	newest = tokens[0].CreatedAt

	for _, token := range tokens {
		if token.CreatedAt.Before(oldest) {
			oldest = token.CreatedAt
		}
		if token.CreatedAt.After(newest) {
			newest = token.CreatedAt
		}
	}

	return oldest, newest
}
