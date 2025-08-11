// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"radar/config"
	"radar/internal/domain/service"
)

// jwtService is a concrete implementation of the TokenService interface using the JWT standard.
type jwtService struct {
	accessSecret  string        // Secret key for signing access tokens.
	refreshSecret string        // Secret key for signing refresh tokens.
	accessTTL     time.Duration // Time-to-live for access tokens.
	refreshTTL    time.Duration // Time-to-live for refresh tokens.
}

// NewJWTService is the constructor for jwtService.
// It takes configuration values to create a new token service instance.
func NewJWTService(cfg *config.Config) (service.TokenService, error) {
	if cfg.SecretKey.Access == "" || cfg.SecretKey.Refresh == "" {
		return nil, errors.New("jwt secrets must be provided")
	}
	return &jwtService{
		accessSecret:  cfg.SecretKey.Access,
		refreshSecret: cfg.SecretKey.Refresh,
		accessTTL:     time.Minute * 15,   // e.g., 15 minutes
		refreshTTL:    time.Hour * 24 * 7, // e.g., 7 days
	}, nil
}

// GenerateTokens creates a new access token and refresh token for a given user and roles.
func (s *jwtService) GenerateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, err error) {
	accessToken, err = s.generateToken(userID, roles, s.accessTTL, s.accessSecret, "access")
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.generateToken(userID, nil, s.refreshTTL, s.refreshSecret, "refresh")
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken checks the validity of a token string against a secret.
func (s *jwtService) ValidateToken(tokenString string, secret string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		// Ensure the signing method is what we expect.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
}

// GetRefreshTokenDuration returns the configured duration for refresh tokens.
func (s *jwtService) GetRefreshTokenDuration() time.Duration {
	return s.refreshTTL
}

// generateToken is a private helper to create a JWT with specific claims.
func (s *jwtService) generateToken(userID uuid.UUID, roles []string, ttl time.Duration, secret, tokenType string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,                     // Subject (who the token is for)
		"iat":  time.Now().Unix(),          // Issued At
		"exp":  time.Now().Add(ttl).Unix(), // Expiration Time
		"type": tokenType,                  // Type of token (access or refresh)
	}
	// Only add roles to the access token for stateless authorization.
	if roles != nil {
		claims["roles"] = roles
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
