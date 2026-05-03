package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess     = "access"
	TokenTypeRefresh    = "refresh"
	TokenTypeOnboarding = "onboarding"
	TokenTypeLinking    = "linking"
)

// Claims defines the custom claims for the JWT tokens.
type Claims struct {
	UserID         uuid.UUID
	Roles          []string
	Type           string
	Provider       string
	ProviderUserID string
	RequestedRole  string
	StoreName      string
	jwt.RegisteredClaims
}

// TokenService defines the interface for generating and validating JWTs.
// This abstracts the details of token creation from the use cases.
type TokenService interface {
	// GenerateTokens creates a new access token and refresh token for a given user.
	GenerateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, err error)

	// ValidateToken checks the validity of a token string.
	ValidateToken(tokenString string) (*Claims, error)

	// GenerateOnboardingToken creates a short-lived token for completing onboarding.
	GenerateOnboardingToken(userID uuid.UUID) (string, error)

	// GenerateLinkingToken creates a short-lived token for confirming OAuth account linking.
	GenerateLinkingToken(
		userID uuid.UUID,
		provider,
		providerUserID,
		requestedRole,
		storeName string,
	) (string, error)

	// GetRefreshTokenDuration returns the configured duration for refresh tokens.
	GetRefreshTokenDuration() time.Duration

	// HashToken creates a SHA-256 hash of the given token for secure storage.
	HashToken(token string) string

	// RotateTokens generates new token pair and returns both tokens with the hash of the refresh token.
	// This method supports the token rotation mechanism for enhanced security.
	RotateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, refreshTokenHash string, err error)
}
