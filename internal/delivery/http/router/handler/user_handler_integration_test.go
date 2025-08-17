package handler

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"
	"radar/internal/infra/auth/google"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestUserHandler_GoogleLogin_Integration(t *testing.T) {
	// Create test config
	testConfig := &config.Config{
		GoogleOAuth: struct {
			ClientID     string `json:"clientId" yaml:"clientId"`
			ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
			RedirectURI  string `json:"redirectUri" yaml:"redirectUri"`
			Scopes       string `json:"scopes" yaml:"scopes"`
		}{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
			RedirectURI:  "http://localhost:8080/oauth/google/callback",
			Scopes:       "openid email profile",
		},
	}

	// Create OAuth service
	oauthService := google.NewOAuthService(testConfig, slog.Default())

	// Create handler with mocked dependencies
	handler := &UserHandler{
		googleOAuthService: oauthService,
	}

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/oauth/google", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call GoogleLogin
	err := handler.GoogleLogin(c)
	assert.NoError(t, err)

	// Check response
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check that the response contains the expected OAuth URL
	responseBody := rec.Body.String()
	assert.Contains(t, responseBody, "test_client_id")

	// The URL will be URL-encoded in the JSON response
	// So we need to check for the encoded version
	assert.Contains(t, responseBody, "http%3A%2F%2Flocalhost%3A8080%2Foauth%2Fgoogle%2Fcallback")
	assert.Contains(t, responseBody, "openid+email+profile")

	// Verify the URL is properly encoded
	assert.Contains(t, responseBody, "client_id=test_client_id")
	assert.Contains(t, responseBody, "redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Foauth%2Fgoogle%2Fcallback")
	assert.Contains(t, responseBody, "scope=openid+email+profile")

	// Check that state parameter is included in response
	assert.Contains(t, responseBody, "state")
}
