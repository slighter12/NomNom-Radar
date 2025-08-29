package google

import (
	"context"
	"fmt"
	"log/slog"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"

	"github.com/pkg/errors"
	"google.golang.org/api/idtoken"
)

type OAuthService struct {
	clientID string
	logger   *slog.Logger
}

// NewOAuthService creates a new Google OAuthService
func NewOAuthService(cfg *config.Config, logger *slog.Logger) service.OAuthAuthService {
	return &OAuthService{
		clientID: cfg.GoogleOAuth.ClientID,
		logger:   logger,
	}
}

// VerifyIDToken implements service.OAuthAuthService interface
func (s *OAuthService) VerifyIDToken(ctx context.Context, idToken string) (*service.OAuthUser, error) {
	// Validate the token using Google's library.
	// This single function handles fetching public keys, verifying the signature,
	// and checking the issuer and expiration time.
	payload, err := idtoken.Validate(ctx, idToken, s.clientID)
	if err != nil {
		s.logger.Error("Google token validation failed", "error", err)

		return nil, errors.Wrap(err, "google token validation failed")
	}

	// After validation, the payload is trustworthy.
	// The library has already checked:
	// 1. The signature is valid and from Google.
	// 2. The token is not expired.
	// 3. The issuer ('iss') is correct.
	// 4. The audience ('aud') matches your clientID.

	claims := payload.Claims

	// Now, you can safely use the claims.
	if emailVerified, ok := claims["email_verified"].(bool); !ok || !emailVerified {
		return nil, fmt.Errorf("email not verified")
	}

	oauthUser := &service.OAuthUser{
		ID:            payload.Subject,
		Email:         claims["email"].(string),
		Name:          claims["name"].(string),
		Provider:      entity.ProviderTypeGoogle,
		AvatarURL:     claims["picture"].(string),
		EmailVerified: true,
		Locale:        claims["locale"].(string),
		ExtraData:     claims,
	}

	s.logger.Info("Google ID token verified successfully",
		slog.String("userID", oauthUser.ID),
		slog.String("email", oauthUser.Email))

	return oauthUser, nil
}

func (s *OAuthService) GetProvider() entity.ProviderType {
	return entity.ProviderTypeGoogle
}
