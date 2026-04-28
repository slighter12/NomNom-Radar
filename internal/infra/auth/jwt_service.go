// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"radar/config"
	"radar/internal/domain/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// jwtService is a concrete implementation of the TokenService interface using the JWT standard.
type jwtService struct {
	accessSecret     string        // Secret key for signing access tokens.
	refreshSecret    string        // Secret key for signing refresh tokens.
	onboardingSecret string        // Secret key for signing onboarding tokens.
	linkingSecret    string        // Secret key for signing account linking tokens.
	onboardingTTL    time.Duration // Time-to-live for onboarding tokens.
	linkingTTL       time.Duration // Time-to-live for account linking tokens.
	accessTTL        time.Duration // Time-to-live for access tokens.
	refreshTTL       time.Duration // Time-to-live for refresh tokens.
}

type linkingTokenMetadata struct {
	Provider        string
	ProviderUserID  string
	RequestedRole   string
	StoreName       string
	BusinessLicense string
}

// NewJWTService is the constructor for jwtService.
// It takes configuration values to create a new token service instance.
func NewJWTService(cfg *config.Config) (service.TokenService, error) {
	if cfg == nil {
		return nil, errors.New("config must be provided")
	}
	config.ApplyDefaults(cfg)

	if cfg.SecretKey.Access == "" || cfg.SecretKey.Refresh == "" {
		return nil, errors.New("jwt secrets must be provided")
	}

	onboardingSecret := cfg.SecretKey.Onboarding
	if onboardingSecret == "" {
		onboardingSecret = deriveTokenSecret(cfg.SecretKey.Access, service.TokenTypeOnboarding)
	}

	linkingSecret := cfg.SecretKey.Linking
	if linkingSecret == "" {
		linkingSecret = deriveTokenSecret(cfg.SecretKey.Access, service.TokenTypeLinking)
	}

	return &jwtService{
		accessSecret:     cfg.SecretKey.Access,
		refreshSecret:    cfg.SecretKey.Refresh,
		onboardingSecret: onboardingSecret,
		linkingSecret:    linkingSecret,
		onboardingTTL:    cfg.Auth.OnboardingTokenTTL,
		linkingTTL:       cfg.Auth.LinkingTokenTTL,
		accessTTL:        cfg.Auth.AccessTokenTTL,
		refreshTTL:       cfg.Auth.RefreshTokenTTL,
	}, nil
}

func deriveTokenSecret(accessSecret, tokenType string) string {
	h := hmac.New(sha256.New, []byte(accessSecret))
	h.Write([]byte(tokenType))

	return hex.EncodeToString(h.Sum(nil))
}

// GenerateTokens creates a new access token and refresh token for a given user and roles.
func (s *jwtService) GenerateTokens(userID uuid.UUID, roles []string) (accessToken string, refreshToken string, err error) {
	accessToken, err = s.generateToken(userID, roles, s.accessTTL, s.accessSecret, service.TokenTypeAccess, nil)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.generateToken(userID, nil, s.refreshTTL, s.refreshSecret, service.TokenTypeRefresh, nil)
	if err != nil {
		return "", "", err
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
	token, err := s.generateToken(userID, nil, s.onboardingTTL, s.onboardingSecret, service.TokenTypeOnboarding, nil)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GenerateLinkingToken creates a short-lived account linking token for a given user and OAuth identity.
func (s *jwtService) GenerateLinkingToken(
	userID uuid.UUID,
	provider,
	providerUserID,
	requestedRole,
	storeName,
	businessLicense string,
) (string, error) {
	token, err := s.generateToken(
		userID,
		nil,
		s.linkingTTL,
		s.linkingSecret,
		service.TokenTypeLinking,
		&linkingTokenMetadata{
			Provider:        provider,
			ProviderUserID:  providerUserID,
			RequestedRole:   requestedRole,
			StoreName:       storeName,
			BusinessLicense: businessLicense,
		},
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

func parseUnverifiedClaims(tokenString string) (*service.Claims, error) {
	unverifiedToken, _, err := new(jwt.Parser).ParseUnverified(tokenString, &service.Claims{})
	if err != nil {
		return nil, fmt.Errorf("parse unverified token claims: %w", err)
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
	case service.TokenTypeLinking:
		return []byte(s.linkingSecret), nil
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
		return nil, fmt.Errorf("parse and validate token claims: %w", err)
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
		return "", "", "", err
	}

	// Hash the refresh token for secure storage
	refreshTokenHash = s.HashToken(refreshToken)

	return accessToken, refreshToken, refreshTokenHash, nil
}

// generateToken is a private helper to create a JWT with specific claims.
func (s *jwtService) generateToken(
	userID uuid.UUID,
	roles []string,
	ttl time.Duration,
	secret, tokenType string,
	linkingMetadata *linkingTokenMetadata,
) (string, error) {
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
	if linkingMetadata != nil {
		claims.Provider = linkingMetadata.Provider
		claims.ProviderUserID = linkingMetadata.ProviderUserID
		claims.RequestedRole = linkingMetadata.RequestedRole
		claims.StoreName = linkingMetadata.StoreName
		claims.BusinessLicense = linkingMetadata.BusinessLicense
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signedToken, nil
}
