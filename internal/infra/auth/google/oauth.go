package google

import (
	"net/url"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"
)

const (
	googleOAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
)

// OAuthService handles Google OAuth infrastructure operations
type OAuthService struct {
	clientID     string
	clientSecret string
	redirectURI  string
	scopes       string
}

// NewOAuthService creates a new Google OAuth service
func NewOAuthService(config *config.Config) service.OAuthService {
	return &OAuthService{
		clientID:     config.GoogleOAuth.ClientID,
		clientSecret: config.GoogleOAuth.ClientSecret,
		redirectURI:  config.GoogleOAuth.RedirectURI,
		scopes:       config.GoogleOAuth.Scopes,
	}
}

// BuildAuthorizationURL constructs the Google OAuth authorization URL
func (s *OAuthService) BuildAuthorizationURL() string {
	params := url.Values{}
	params.Set("client_id", s.clientID)
	params.Set("redirect_uri", s.redirectURI)
	params.Set("scope", s.scopes)
	params.Set("response_type", "code")

	return googleOAuthURL + "?" + params.Encode()
}

// GetProvider returns the OAuth provider type
func (s *OAuthService) GetProvider() entity.ProviderType {
	return entity.ProviderTypeGoogle
}

// ToDomainConfig converts internal config to domain config
func (s *OAuthService) ToDomainConfig() service.OAuthConfig {
	return service.OAuthConfig{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		RedirectURI:  s.redirectURI,
		Scopes:       s.scopes,
		Provider:     entity.ProviderTypeGoogle,
	}
}
