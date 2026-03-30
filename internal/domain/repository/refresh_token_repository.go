package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// RefreshTokenRepository defines the interface for refresh token and session management operations.
// This supports multi-device login and remote logout functionality.
type RefreshTokenRepository interface {
	// CreateRefreshToken persists a new refresh token, representing a user session.
	CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error

	// FindRefreshTokenByHash retrieves a refresh token record by its securely stored hash.
	FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*entity.RefreshToken, error)

	// FindRefreshTokenByID retrieves a refresh token record by its unique ID.
	FindRefreshTokenByID(ctx context.Context, id uuid.UUID) (*entity.RefreshToken, error)

	// FindRefreshTokensByUserID retrieves all active refresh tokens for a specific user.
	// This allows users to see all their active sessions across different devices.
	FindRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error)

	// UpdateRefreshToken updates an existing refresh token record.
	// This can be used to extend expiration or update device information.
	UpdateRefreshToken(ctx context.Context, token *entity.RefreshToken) error

	// DeleteRefreshToken removes a refresh token by its ID, effectively ending a session.
	DeleteRefreshToken(ctx context.Context, id uuid.UUID) error

	// DeleteRefreshTokenByHash deletes a refresh token by its hash, effectively ending a session.
	DeleteRefreshTokenByHash(ctx context.Context, tokenHash string) error

	// DeleteRefreshTokensByUserID removes all refresh tokens for a specific user.
	// This is useful for "logout from all devices" functionality.
	DeleteRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error

	// DeleteExpiredRefreshTokens removes all expired refresh tokens from the database.
	// This should be called periodically for cleanup.
	DeleteExpiredRefreshTokens(ctx context.Context) error

	// CountActiveSessionsByUserID returns the number of active (non-expired) sessions for a user.
	// This can be used to implement session limits or monitoring.
	CountActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) (int, error)
}
