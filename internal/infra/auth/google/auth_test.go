package google

import (
	"context"
	"log/slog"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"

	"github.com/stretchr/testify/assert"
)

func TestAuthService_VerifyToken(t *testing.T) {
	logger := slog.Default()
	config := &config.Config{}
	config.GoogleOAuth.ClientID = "test_client_id"
	authService := NewAuthService(config, logger)
	ctx := context.Background()

	// Test with a mock JWT token (this will fail validation but not parsing)
	mockJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0X3VzZXJfMTIzIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTYzNTU5NzIwMCwiZXhwIjoxNjM1NjgzNjAwLCJhdWQiOiJ0ZXN0X2NsaWVudF9pZCIsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlfQ.invalid_signature"

	// This should fail validation but not parsing
	oauthUser, err := authService.VerifyToken(ctx, mockJWT)
	assert.Error(t, err) // Should fail validation
	assert.Nil(t, oauthUser)
	assert.Contains(t, err.Error(), "token verification failed")
}

func TestAuthService_GetProvider(t *testing.T) {
	logger := slog.Default()
	config := &config.Config{}
	config.GoogleOAuth.ClientID = "test_client_id"
	authService := NewAuthService(config, logger)
	provider := authService.GetProvider()

	assert.Equal(t, entity.ProviderTypeGoogle, provider)
}

func TestAuthService_ParseIDToken(t *testing.T) {
	logger := slog.Default()
	config := &config.Config{}
	config.GoogleOAuth.ClientID = "test_client_id"
	authService := NewAuthService(config, logger)

	// Test valid JWT format
	validJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0X3VzZXJfMTIzIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTYzNTU5NzIwMCwiZXhwIjoxNjM1NjgzNjAwLCJhdWQiOiJ0ZXN0X2NsaWVudF9pZCIsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlfQ.invalid_signature"

	// Cast to concrete type to test internal method
	authServiceImpl := authService.(*AuthServiceImpl)
	claims, err := authServiceImpl.parseIDToken(validJWT)

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "test_user_123", claims.Sub)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "Test User", claims.Name)
	assert.Equal(t, "test_client_id", claims.Aud)
	assert.Equal(t, "https://accounts.google.com", claims.Iss)
	assert.True(t, claims.EmailVerified)
}

func TestAuthService_InvalidJWT(t *testing.T) {
	logger := slog.Default()
	config := &config.Config{}
	config.GoogleOAuth.ClientID = "test_client_id"
	authService := NewAuthService(config, logger)
	ctx := context.Background()

	// Test invalid JWT format
	invalidJWT := "invalid_token_format"

	oauthUser, err := authService.VerifyToken(ctx, invalidJWT)
	assert.Error(t, err)
	assert.Nil(t, oauthUser)
	assert.Contains(t, err.Error(), "invalid JWT format")
}
