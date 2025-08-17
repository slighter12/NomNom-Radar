// Package usecase contains the application-specific business rules.
package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// SessionUsecase defines the interface for session management operations.
type SessionUsecase interface {
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.SessionInfo, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error
	RevokeAllOtherSessions(ctx context.Context, userID uuid.UUID, currentSessionID uuid.UUID) error
	GetSessionInfo(ctx context.Context, userID, sessionID uuid.UUID) (*entity.SessionInfo, error)
	CleanupExpiredSessions(ctx context.Context) (int, error)
	DetectAnomalousActivity(ctx context.Context, userID uuid.UUID) ([]*entity.AnomalousActivity, error)
	GetSessionStatistics(ctx context.Context, userID uuid.UUID) (*entity.SessionStatistics, error)
}
