package google

import (
	"testing"

	"radar/config"
	"radar/internal/domain/entity"

	"github.com/stretchr/testify/assert"
)

func TestOAuthService_BuildAuthorizationURL(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		config   *config.Config
		expected string
	}{
		{
			name: "basic config",
			config: &config.Config{
				GoogleOAuth: struct {
					ClientID     string `json:"clientId" yaml:"clientId"`
					ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
					RedirectURI  string `json:"redirectUri" yaml:"redirectUri"`
					Scopes       string `json:"scopes" yaml:"scopes"`
				}{
					ClientID:     "test_client_id",
					ClientSecret: "test_secret",
					RedirectURI:  "http://localhost:8080/callback",
					Scopes:       "openid email profile",
				},
			},
			expected: "https://accounts.google.com/o/oauth2/v2/auth?client_id=test_client_id&redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fcallback&response_type=code&scope=openid+email+profile",
		},
		{
			name: "with special characters in scopes",
			config: &config.Config{
				GoogleOAuth: struct {
					ClientID     string `json:"clientId" yaml:"clientId"`
					ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
					RedirectURI  string `json:"redirectUri" yaml:"redirectUri"`
					Scopes       string `json:"scopes" yaml:"scopes"`
				}{
					ClientID:     "test_client_id",
					ClientSecret: "test_secret",
					RedirectURI:  "http://localhost:8080/callback",
					Scopes:       "openid email profile https://www.googleapis.com/auth/userinfo.profile",
				},
			},
			expected: "https://accounts.google.com/o/oauth2/v2/auth?client_id=test_client_id&redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fcallback&response_type=code&scope=openid+email+profile+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOAuthService(tt.config)
			result := service.BuildAuthorizationURL()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOAuthService_GetProvider(t *testing.T) {
	config := &config.Config{
		GoogleOAuth: struct {
			ClientID     string `json:"clientId" yaml:"clientId"`
			ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
			RedirectURI  string `json:"redirectUri" yaml:"redirectUri"`
			Scopes       string `json:"scopes" yaml:"scopes"`
		}{
			ClientID:     "test_client_id",
			ClientSecret: "test_secret",
			RedirectURI:  "http://localhost:8080/callback",
			Scopes:       "openid email profile",
		},
	}

	service := NewOAuthService(config)
	provider := service.GetProvider()

	assert.Equal(t, entity.ProviderTypeGoogle, provider)
}
