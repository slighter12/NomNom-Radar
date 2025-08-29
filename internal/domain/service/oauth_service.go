package service

import (
	"context"

	"radar/internal/domain/entity"
)

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

// OAuthAuthService defines the interface for OAuth authentication operations
// This is specifically for ID token verification (like Google ID tokens)
type OAuthAuthService interface {
	// VerifyIDToken verifies an OAuth ID token and returns user information
	// This is primarily used for Google Sign-In where the client sends an ID token directly
	VerifyIDToken(ctx context.Context, idToken string) (*OAuthUser, error)

	// GetProvider returns the OAuth provider type
	GetProvider() entity.ProviderType
}
