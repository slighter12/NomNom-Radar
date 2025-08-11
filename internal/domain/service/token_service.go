package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenService defines the interface for generating and validating JWTs.
// This abstracts the details of token creation from the use cases.
type TokenService interface {
	// GenerateTokens creates a new access token and refresh token for a given user.
	GenerateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, err error)

	// ValidateToken checks the validity of a token string against a secret.
	ValidateToken(tokenString string, secret string) (*jwt.Token, error)

	// GetRefreshTokenDuration returns the configured duration for refresh tokens.
	GetRefreshTokenDuration() time.Duration
}
