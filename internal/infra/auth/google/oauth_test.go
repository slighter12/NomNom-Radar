package google

import (
	"context"
	"log/slog"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthService_VerifyIDToken(t *testing.T) {
	logger := slog.Default()
	cfg := &config.Config{
		GoogleOAuth: &config.GoogleOAuthConfig{
			ClientID: "test_client_id",
		},
	}
	authService, err := NewOAuthService(cfg, logger)
	require.NoError(t, err)
	ctx := context.Background()

	// Test with a mock JWT token (this will fail validation but not parsing)
	mockJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0X3VzZXJfMTIzIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTYzNTU5NzIwMCwiZXhwIjoxNjM1NjgzNjAwLCJhdWQiOiJ0ZXN0X2NsaWVudF9pZCIsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlfQ.invalid_signature"

	// This should fail validation but not parsing
	oauthUser, err := authService.VerifyIDToken(ctx, mockJWT)
	assert.Error(t, err) // Should fail validation
	assert.Nil(t, oauthUser)
	assert.Contains(t, err.Error(), "unable to decode JWT signature")
}

func TestAuthService_GetProvider(t *testing.T) {
	logger := slog.Default()
	cfg := &config.Config{
		GoogleOAuth: &config.GoogleOAuthConfig{
			ClientID: "test_client_id",
		},
	}
	authService, err := NewOAuthService(cfg, logger)
	require.NoError(t, err)
	provider := authService.GetProvider()

	assert.Equal(t, entity.ProviderTypeGoogle, provider)
}

func TestNewOAuthService_RequiresConfig(t *testing.T) {
	logger := slog.Default()

	authService, err := NewOAuthService(nil, logger)

	require.Error(t, err)
	assert.Nil(t, authService)
	assert.Contains(t, err.Error(), "google oauth config is required")
}

func TestNewOAuthService_RequiresGoogleOAuthSection(t *testing.T) {
	logger := slog.Default()

	authService, err := NewOAuthService(&config.Config{}, logger)

	require.Error(t, err)
	assert.Nil(t, authService)
	assert.Contains(t, err.Error(), "google oauth config is required")
}

func TestNewOAuthService_RequiresClientID(t *testing.T) {
	logger := slog.Default()
	cfg := &config.Config{
		GoogleOAuth: &config.GoogleOAuthConfig{
			ClientID: "   ",
		},
	}

	authService, err := NewOAuthService(cfg, logger)

	require.Error(t, err)
	assert.Nil(t, authService)
	assert.Contains(t, err.Error(), "google oauth client_id is required")
}
