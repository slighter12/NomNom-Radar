package google

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"

	"github.com/pkg/errors"
)

const (
	googleOAuthURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// OAuthService handles Google OAuth infrastructure operations
type OAuthService struct {
	clientID     string
	clientSecret string
	redirectURI  string
	scopes       string

	// State storage for CSRF protection
	stateStore map[string]time.Time
	stateMutex sync.RWMutex
}

// NewOAuthService creates a new Google OAuth service
func NewOAuthService(config *config.Config) service.OAuthService {
	return &OAuthService{
		clientID:     config.GoogleOAuth.ClientID,
		clientSecret: config.GoogleOAuth.ClientSecret,
		redirectURI:  config.GoogleOAuth.RedirectURI,
		scopes:       config.GoogleOAuth.Scopes,
		stateStore:   make(map[string]time.Time),
	}
}

// generateState generates a cryptographically secure random state string
func (s *OAuthService) generateState() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// storeState stores a state parameter with expiration time
func (s *OAuthService) storeState(state string) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	// State expires after 10 minutes
	s.stateStore[state] = time.Now().Add(10 * time.Minute)

	// Clean up expired states
	s.cleanupExpiredStates()
}

// cleanupExpiredStates removes expired state parameters
func (s *OAuthService) cleanupExpiredStates() {
	now := time.Now()
	for state, expiry := range s.stateStore {
		if now.After(expiry) {
			delete(s.stateStore, state)
		}
	}
}

// BuildAuthorizationURL constructs the Google OAuth authorization URL with state parameter for CSRF protection
func (s *OAuthService) BuildAuthorizationURL(state string) string {
	// Store the state parameter for later validation
	s.storeState(state)

	params := url.Values{}
	params.Set("client_id", s.clientID)
	params.Set("redirect_uri", s.redirectURI)
	params.Set("scope", s.scopes)
	params.Set("response_type", "code")
	params.Set("state", state)

	return googleOAuthURL + "?" + params.Encode()
}

// ValidateState validates the state parameter to prevent CSRF attacks
func (s *OAuthService) ValidateState(state string) bool {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	expiry, exists := s.stateStore[state]
	if !exists {
		return false
	}

	// Check if state has expired
	if time.Now().After(expiry) {
		// Remove expired state
		s.stateMutex.RUnlock()
		s.stateMutex.Lock()
		delete(s.stateStore, state)
		s.stateMutex.Unlock()
		s.stateMutex.RLock()
		return false
	}

	// Remove used state to prevent replay attacks
	s.stateMutex.RUnlock()
	s.stateMutex.Lock()
	delete(s.stateStore, state)
	s.stateMutex.Unlock()
	s.stateMutex.RLock()

	return true
}

// GetProvider returns the OAuth provider type
func (s *OAuthService) GetProvider() entity.ProviderType {
	return entity.ProviderTypeGoogle
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (s *OAuthService) ExchangeCodeForToken(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", s.clientID)
	data.Set("client_secret", s.clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", s.redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", errors.Wrap(err, "failed to create token exchange request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to exchange code for token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", errors.Wrap(err, "failed to decode token response")
	}

	return tokenResponse.AccessToken, nil
}

// GetUserInfo retrieves user information using an access token
func (s *OAuthService) GetUserInfo(ctx context.Context, accessToken string) (*service.OAuthUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", googleUserInfoURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create user info request")
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("user info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
		Locale        string `json:"locale"`
		Link          string `json:"link"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, errors.Wrap(err, "failed to decode user info response")
	}

	return &service.OAuthUser{
		ID:            googleUser.ID,
		Email:         googleUser.Email,
		Name:          googleUser.Name,
		Provider:      entity.ProviderTypeGoogle,
		ProfileURL:    googleUser.Link,
		AvatarURL:     googleUser.Picture,
		EmailVerified: googleUser.VerifiedEmail,
		Locale:        googleUser.Locale,
		ExtraData:     make(map[string]interface{}),
	}, nil
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
