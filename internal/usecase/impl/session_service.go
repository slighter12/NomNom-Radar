package impl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	deliverycontext "radar/internal/delivery/context"
	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

// sessionService implements the SessionUsecase interface.
type sessionService struct {
	fx.In

	txManager          repository.TransactionManager
	logger             *slog.Logger
	refreshTokenPolicy policy.RefreshTokenPolicy
}

// NewSessionService is the constructor for sessionService.
func NewSessionService(
	txManager repository.TransactionManager,
	logger *slog.Logger,
) usecase.SessionUsecase {
	return &sessionService{
		txManager:          txManager,
		logger:             logger,
		refreshTokenPolicy: policy.DefaultRefreshTokenPolicy(),
	}
}

// log returns a request-scoped logger if available, otherwise falls back to the service's logger.
func (srv *sessionService) log(ctx context.Context) *slog.Logger {
	return deliverycontext.GetLoggerOrDefault(ctx, srv.logger)
}

// GetActiveSessions retrieves all active sessions for a user.
func (srv *sessionService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.SessionInfo, error) {
	srv.log(ctx).Debug("Getting active sessions", slog.Any("user_id", userID))

	var sessions []*entity.SessionInfo

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		refreshRepo := repoFactory.RefreshTokenRepo()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 2. Get all refresh tokens for the user
		tokens, err := refreshRepo.FindRefreshTokensByUserID(ctx, userID)
		if err != nil {
			return err
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
		srv.log(ctx).Error("Failed to get active sessions", slog.Any("error", err), slog.Any("user_id", userID))

		return nil, err
	}
	srv.log(ctx).Debug("Successfully retrieved active sessions", slog.Any("user_id", userID), slog.Int("count", len(sessions)))

	return sessions, nil
}

// RevokeSession revokes a specific session.
func (srv *sessionService) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	srv.log(ctx).Info("Revoking session", slog.Any("user_id", userID), slog.Any("session_id", sessionID))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		refreshRepo := repoFactory.RefreshTokenRepo()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 2. Find the session
		token, err := refreshRepo.FindRefreshTokenByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrRefreshTokenNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 3. Verify ownership
		if token.UserID != userID {
			return fmt.Errorf("session does not belong to user: %w", domainerrors.ErrForbidden)
		}

		// 4. Delete the session
		if err := refreshRepo.DeleteRefreshToken(ctx, sessionID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to revoke session", slog.Any("error", err), slog.Any("user_id", userID), slog.Any("session_id", sessionID))

		return err
	}
	srv.log(ctx).Info("Successfully revoked session", slog.Any("user_id", userID), slog.Any("session_id", sessionID))

	return nil
}

// RevokeAllSessions revokes all refresh token families for a user.
func (srv *sessionService) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	srv.log(ctx).Info("Revoking all sessions", slog.Any("user_id", userID))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		refreshRepo := repoFactory.RefreshTokenRepo()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 2. Revoke all sessions while retaining reuse-detection records until cleanup
		if err := refreshRepo.RevokeTokenFamiliesByUserID(ctx, userID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to revoke all sessions", slog.Any("error", err), slog.Any("user_id", userID))

		return err
	}
	srv.log(ctx).Info("Successfully revoked all sessions", slog.Any("user_id", userID))

	return nil
}

// RevokeAllOtherSessions revokes all sessions except the current one.
func (srv *sessionService) RevokeAllOtherSessions(ctx context.Context, userID uuid.UUID, currentSessionID uuid.UUID) error {
	srv.log(ctx).Info("Revoking all other sessions", slog.Any("user_id", userID), slog.Any("current_session_id", currentSessionID))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		refreshRepo := repoFactory.RefreshTokenRepo()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 2. Get all sessions
		tokens, err := refreshRepo.FindRefreshTokensByUserID(ctx, userID)
		if err != nil {
			return err
		}

		// 3. Delete all sessions except the current one
		for _, token := range tokens {
			if token.ID != currentSessionID {
				if err := refreshRepo.DeleteRefreshToken(ctx, token.ID); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to revoke all other sessions", slog.Any("error", err), slog.Any("user_id", userID))

		return err
	}
	srv.log(ctx).Info("Successfully revoked all other sessions", slog.Any("user_id", userID), slog.Any("current_session_id", currentSessionID))

	return nil
}

// GetSessionInfo retrieves detailed information about a specific session.
func (srv *sessionService) GetSessionInfo(ctx context.Context, userID, sessionID uuid.UUID) (*entity.SessionInfo, error) {
	srv.log(ctx).Debug("Getting session info", slog.Any("user_id", userID), slog.Any("session_id", sessionID))

	var sessionInfo *entity.SessionInfo

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		refreshRepo := repoFactory.RefreshTokenRepo()

		// 1. Verify user exists
		_, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 2. Find the session
		token, err := refreshRepo.FindRefreshTokenByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrRefreshTokenNotFound) {
				return domainerrors.ErrNotFound
			}

			return err
		}

		// 3. Verify ownership
		if token.UserID != userID {
			return fmt.Errorf("session does not belong to user: %w", domainerrors.ErrForbidden)
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
		srv.log(ctx).Error("Failed to get session info", slog.Any("error", err), slog.Any("user_id", userID), slog.Any("session_id", sessionID))

		return nil, err
	}
	srv.log(ctx).Debug("Successfully retrieved session info", slog.Any("user_id", userID), slog.Any("session_id", sessionID))

	return sessionInfo, nil
}

// CleanupExpiredSessions removes all expired sessions from the database.
func (srv *sessionService) CleanupExpiredSessions(ctx context.Context) (int, error) {
	srv.log(ctx).Info("Cleaning up expired sessions")

	var deletedCount int

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		refreshRepo := repoFactory.RefreshTokenRepo()

		// Delete expired sessions
		if err := refreshRepo.DeleteExpiredRefreshTokens(ctx, srv.refreshTokenPolicy.RevokedRetentionDays); err != nil {
			return fmt.Errorf("failed to delete expired sessions: %w", err)
		}

		// Note: In a real implementation, you might want to return the count
		// from the repository method
		deletedCount = 0 // Placeholder

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to cleanup expired sessions", slog.Any("error", err))

		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	srv.log(ctx).Info("Successfully cleaned up expired sessions", slog.Int("deleted_count", deletedCount))

	return deletedCount, nil
}
