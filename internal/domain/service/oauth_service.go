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
	ID            string              // Provider-specific user ID (e.g., Google's 'sub' claim)
	Email         string              // User's email address
	Name          string              // User's display name
	Provider      entity.ProviderType // The OAuth provider (google, apple, etc.)
	ProfileURL    string              // URL to user's profile page
	AvatarURL     string              // URL to user's profile picture
	EmailVerified bool                // Whether the email is verified by the provider
	Locale        string              // User's locale/language preference
	ExtraData     map[string]any      // Additional provider-specific data
}

// GoogleUserInfo represents specific Google user information from ID token
type GoogleUserInfo struct {
	Sub           string // Google user ID
	Email         string // User's email
	Name          string // User's full name
	GivenName     string // User's first name
	FamilyName    string // User's last name
	Picture       string // Profile picture URL
	EmailVerified bool   // Whether email is verified
	Locale        string // User's locale
}

// OAuthService defines the interface for OAuth operations
type OAuthService interface {
	// BuildAuthorizationURL constructs the OAuth authorization URL with state parameter for CSRF protection
	BuildAuthorizationURL(state string) string

	// GetProvider returns the OAuth provider type
	GetProvider() entity.ProviderType

	// ExchangeCodeForToken exchanges an authorization code for an access token
	ExchangeCodeForToken(ctx context.Context, code string) (string, error)

	// GetUserInfo retrieves user information using an access token
	GetUserInfo(ctx context.Context, accessToken string) (*OAuthUser, error)

	// ValidateState validates the state parameter to prevent CSRF attacks
	ValidateState(state string) bool
}

// OAuthAuthService defines the interface for OAuth authentication operations
// This is specifically for ID token verification (like Google ID tokens)
type OAuthAuthService interface {
	// VerifyIDToken verifies an OAuth ID token and returns user information
	// This is primarily used for Google Sign-In where the client sends an ID token directly
	VerifyIDToken(ctx context.Context, idToken string) (*OAuthUser, error)

	// VerifyGoogleIDToken specifically verifies a Google ID token and returns Google user info
	// This method provides more detailed Google-specific information
	VerifyGoogleIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error)

	// GetProvider returns the OAuth provider type
	GetProvider() entity.ProviderType

	// ValidateTokenAudience validates that the token was issued for the correct client ID
	ValidateTokenAudience(ctx context.Context, idToken string, expectedClientID string) error
}
