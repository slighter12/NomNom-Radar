package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims defines the custom claims for the JWT tokens.
type Claims struct {
	UserID uuid.UUID
	Roles  []string
	Type   string // Add this field
	jwt.RegisteredClaims
}

// TokenService defines the interface for generating and validating JWTs.
// This abstracts the details of token creation from the use cases.
type TokenService interface {
	// GenerateTokens creates a new access token and refresh token for a given user.
	GenerateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, err error)

	// ValidateToken checks the validity of a token string.
	ValidateToken(tokenString string) (*Claims, error)

	// GetRefreshTokenDuration returns the configured duration for refresh tokens.
	GetRefreshTokenDuration() time.Duration
}
