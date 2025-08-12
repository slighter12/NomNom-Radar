package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"

	"github.com/pkg/errors"
)

// GoogleIDTokenClaims represents the claims in a Google ID token
type GoogleIDTokenClaims struct {
	Iss           string `json:"iss"`            // Issuer
	Sub           string `json:"sub"`            // Subject (user ID)
	Aud           string `json:"aud"`            // Audience (client ID)
	Exp           int64  `json:"exp"`            // Expiration time
	Iat           int64  `json:"iat"`            // Issued at
	Email         string `json:"email"`          // User's email
	EmailVerified bool   `json:"email_verified"` // Email verification status
	Name          string `json:"name"`           // User's full name
	Picture       string `json:"picture"`        // User's profile picture
	GivenName     string `json:"given_name"`     // First name
	FamilyName    string `json:"family_name"`    // Last name
}

// AuthServiceImpl implements the Google AuthService and service.OAuthAuthService
type AuthServiceImpl struct {
	clientID string
	logger   *slog.Logger
}

// NewAuthService creates a new Google AuthService
func NewAuthService(cfg *config.Config, logger *slog.Logger) service.OAuthAuthService {
	return &AuthServiceImpl{
		clientID: cfg.GoogleOAuth.ClientID,
		logger:   logger,
	}
}

// VerifyToken implements service.OAuthAuthService interface
func (s *AuthServiceImpl) VerifyToken(ctx context.Context, token string) (*service.OAuthUser, error) {
	s.logger.Info("Verifying Google ID token", "clientID", s.clientID)

	// Parse the JWT token to get claims
	claims, err := s.parseIDToken(token)
	if err != nil {
		s.logger.Error("Failed to parse ID token", "error", err)

		return nil, errors.Wrap(err, "invalid ID token")
	}

	// Verify the token
	if err := s.verifyTokenClaims(claims); err != nil {
		s.logger.Error("Token verification failed", "error", err)

		return nil, errors.Wrap(err, "token verification failed")
	}

	// Convert to OAuth user
	oauthUser := &service.OAuthUser{
		ID:         claims.Sub,
		Email:      claims.Email,
		Name:       claims.Name,
		Provider:   entity.ProviderTypeGoogle,
		ProfileURL: "", // Can be added later
		AvatarURL:  claims.Picture,
	}

	s.logger.Info("Google ID token verified successfully",
		slog.String("userID", oauthUser.ID),
		slog.String("email", oauthUser.Email))

	return oauthUser, nil
}

// GetProvider returns the OAuth provider type
func (s *AuthServiceImpl) GetProvider() entity.ProviderType {
	return entity.ProviderTypeGoogle
}

// parseIDToken parses the JWT token and extracts claims
func (s *AuthServiceImpl) parseIDToken(token string) (*GoogleIDTokenClaims, error) {
	// Split the token into parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	// Decode the payload (second part)
	payload := parts[1]

	// Add padding if needed
	if len(payload)%4 != 0 {
		payload += strings.Repeat("=", 4-len(payload)%4)
	}

	// Decode base64
	decoded, err := base64Decode(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode token payload")
	}

	// Parse JSON claims
	var claims GoogleIDTokenClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, errors.Wrap(err, "failed to parse token claims")
	}

	return &claims, nil
}

// verifyTokenClaims verifies the token claims
func (s *AuthServiceImpl) verifyTokenClaims(claims *GoogleIDTokenClaims) error {
	// Check issuer
	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
		return errors.Errorf("invalid issuer: %s", claims.Iss)
	}

	// Check audience (client ID)
	if claims.Aud != s.clientID {
		return errors.Errorf("invalid audience: expected %s, got %s", s.clientID, claims.Aud)
	}

	// Check expiration
	now := time.Now().Unix()
	if claims.Exp < now {
		return errors.Errorf("token expired: expired at %d, current time %d", claims.Exp, now)
	}

	// Check email verification
	if !claims.EmailVerified {
		return errors.New("email not verified")
	}

	return nil
}

// base64Decode decodes base64 URL-safe string
func base64Decode(str string) ([]byte, error) {
	// Replace URL-safe characters
	str = strings.ReplaceAll(str, "-", "+")
	str = strings.ReplaceAll(str, "_", "/")

	// Add padding if needed
	if len(str)%4 != 0 {
		str += strings.Repeat("=", 4-len(str)%4)
	}

	// Decode
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode base64 string")
	}

	return decoded, nil
}
