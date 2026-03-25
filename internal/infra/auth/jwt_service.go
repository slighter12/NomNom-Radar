// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"radar/config"
	"radar/internal/domain/service"
	"radar/internal/errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// jwtService is a concrete implementation of the TokenService interface using the JWT standard.
type jwtService struct {
	accessSecret     string        // Secret key for signing access tokens.
	refreshSecret    string        // Secret key for signing refresh tokens.
	onboardingSecret string        // Secret key for signing onboarding tokens.
	onboardingTTL    time.Duration // Time-to-live for onboarding tokens.
	accessTTL        time.Duration // Time-to-live for access tokens.
	refreshTTL       time.Duration // Time-to-live for refresh tokens.
}

// NewJWTService is the constructor for jwtService.
// It takes configuration values to create a new token service instance.
func NewJWTService(cfg *config.Config) (service.TokenService, error) {
	if cfg.SecretKey.Access == "" || cfg.SecretKey.Refresh == "" {
		return nil, errors.New("jwt secrets must be provided")
	}

	onboardingSecret := cfg.SecretKey.Onboarding
	if onboardingSecret == "" {
		h := hmac.New(sha256.New, []byte(cfg.SecretKey.Access))
		h.Write([]byte(service.TokenTypeOnboarding))
		onboardingSecret = hex.EncodeToString(h.Sum(nil))
	}

	return &jwtService{
		accessSecret:     cfg.SecretKey.Access,
		refreshSecret:    cfg.SecretKey.Refresh,
		onboardingSecret: onboardingSecret,
		onboardingTTL:    time.Minute * 10,
		accessTTL:        time.Minute * 15,   // e.g., 15 minutes
		refreshTTL:       time.Hour * 24 * 7, // e.g., 7 days
	}, nil
}

// GenerateTokens creates a new access token and refresh token for a given user and roles.
func (s *jwtService) GenerateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, err error) {
	accessToken, err = s.generateToken(userID, roles, s.accessTTL, s.accessSecret, service.TokenTypeAccess)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	refreshToken, err = s.generateToken(userID, nil, s.refreshTTL, s.refreshSecret, service.TokenTypeRefresh)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	return accessToken, refreshToken, nil
}

// ValidateToken checks the validity of a token string against a secret.
func (s *jwtService) ValidateToken(tokenString string) (*service.Claims, error) {
	unverifiedClaims, err := parseUnverifiedClaims(tokenString)
	if err != nil {
		return nil, err
	}

	secret, err := s.secretForTokenType(unverifiedClaims.Type)
	if err != nil {
		return nil, err
	}

	return parseAndValidateClaims(tokenString, secret)
}

// GenerateOnboardingToken creates a short-lived onboarding token for a given user.
func (s *jwtService) GenerateOnboardingToken(userID uuid.UUID) (string, error) {
	token, err := s.generateToken(userID, nil, s.onboardingTTL, s.onboardingSecret, service.TokenTypeOnboarding)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return token, nil
}

func parseUnverifiedClaims(tokenString string) (*service.Claims, error) {
	unverifiedToken, _, err := new(jwt.Parser).ParseUnverified(tokenString, &service.Claims{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	unverifiedClaims, ok := unverifiedToken.Claims.(*service.Claims)
	if !ok {
		return nil, errors.New("invalid token claims structure")
	}

	return unverifiedClaims, nil
}

func (s *jwtService) secretForTokenType(tokenType string) ([]byte, error) {
	switch tokenType {
	case service.TokenTypeAccess:
		return []byte(s.accessSecret), nil
	case service.TokenTypeRefresh:
		return []byte(s.refreshSecret), nil
	case service.TokenTypeOnboarding:
		return []byte(s.onboardingSecret), nil
	default:
		return nil, errors.New("unknown token type")
	}
}

func parseAndValidateClaims(tokenString string, secret []byte) (*service.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &service.Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return secret, nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	claims, ok := token.Claims.(*service.Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// GetRefreshTokenDuration returns the configured duration for refresh tokens.
func (s *jwtService) GetRefreshTokenDuration() time.Duration {
	return s.refreshTTL
}

// HashToken creates a SHA-256 hash of the given token for secure storage.
// This is used to store refresh tokens securely in the database.
func (s *jwtService) HashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))

	return hex.EncodeToString(hasher.Sum(nil))
}

// RotateTokens generates new token pair and returns both tokens with the hash of the refresh token.
// This method supports the token rotation mechanism for enhanced security.
// Each time tokens are rotated, both access and refresh tokens are regenerated.
func (s *jwtService) RotateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, refreshTokenHash string, err error) {
	// Generate new token pair
	accessToken, refreshToken, err = s.GenerateTokens(userID, roles)
	if err != nil {
		return "", "", "", errors.WithStack(err)
	}

	// Hash the refresh token for secure storage
	refreshTokenHash = s.HashToken(refreshToken)

	return accessToken, refreshToken, refreshTokenHash, nil
}

// generateToken is a private helper to create a JWT with specific claims.
func (s *jwtService) generateToken(userID uuid.UUID, roles []string, ttl time.Duration, secret, tokenType string) (string, error) {
	claims := service.Claims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
			ID:        uuid.NewString(),
		},
		Type: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", errors.WithStack(err)
	}

	return signedToken, nil
}
