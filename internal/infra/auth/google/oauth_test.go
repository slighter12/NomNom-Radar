package google

import (
	"context"
	"log/slog"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"

	"github.com/stretchr/testify/assert"
)

func TestAuthService_VerifyIDToken(t *testing.T) {
	logger := slog.Default()
	config := &config.Config{
		GoogleOAuth: &config.GoogleOAuthConfig{
			ClientID: "test_client_id",
		},
	}
	authService := NewOAuthService(config, logger)
	ctx := context.Background()

	// Test with a mock JWT token (this will fail validation but not parsing)
	mockJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0X3VzZXJfMTIzIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTYzNTU5NzIwMCwiZXhwIjoxNjM1NjgzNjAwLCJhdWQiOiJ0ZXN0X2NsaWVudF9pZCIsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlfQ.invalid_signature"

	// This should fail validation but not parsing
	oauthUser, err := authService.VerifyIDToken(ctx, mockJWT)
	assert.Error(t, err) // Should fail validation
	assert.Nil(t, oauthUser)
	assert.Contains(t, err.Error(), "token verification failed")
}

func TestAuthService_GetProvider(t *testing.T) {
	logger := slog.Default()
	config := &config.Config{
		GoogleOAuth: &config.GoogleOAuthConfig{
			ClientID: "test_client_id",
		},
	}
	authService := NewOAuthService(config, logger)
	provider := authService.GetProvider()

	assert.Equal(t, entity.ProviderTypeGoogle, provider)
}
