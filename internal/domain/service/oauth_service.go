package service

import (
	"context"

	"radar/internal/domain/entity"
)

// OAuthConfig holds the configuration for OAuth services
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       string
	Provider     entity.ProviderType
}

// OAuthUser represents user information from OAuth providers
type OAuthUser struct {
	ID         string
	Email      string
	Name       string
	Provider   entity.ProviderType
	ProfileURL string
	AvatarURL  string
}

// OAuthService defines the interface for OAuth operations
type OAuthService interface {
	// BuildAuthorizationURL constructs the OAuth authorization URL
	BuildAuthorizationURL() string

	// GetProvider returns the OAuth provider type
	GetProvider() entity.ProviderType
}

// OAuthAuthService defines the interface for OAuth authentication operations
type OAuthAuthService interface {
	// VerifyToken verifies an OAuth token and returns user information
	VerifyToken(ctx context.Context, token string) (*OAuthUser, error)

	// GetProvider returns the OAuth provider type
	GetProvider() entity.ProviderType
}
