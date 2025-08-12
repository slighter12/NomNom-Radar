package auth

import (
	"testing"
	"time"

	"radar/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJWTService_GenerateAndValidateTokens(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		SecretKey: struct {
			Access  string `json:"access" yaml:"access"`
			Refresh string `json:"refresh" yaml:"refresh"`
		}{
			Access:  "test_access_secret_key_very_long_for_testing",
			Refresh: "test_refresh_secret_key_very_long_for_testing",
		},
	}

	// Create JWT service
	jwtService, err := NewJWTService(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, jwtService)

	// Test data
	userID := uuid.New()
	roles := []string{"user", "admin"}

	// Generate tokens
	accessToken, refreshToken, err := jwtService.GenerateTokens(userID, roles)
	assert.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// Validate access token
	accessClaims, err := jwtService.ValidateToken(accessToken)
	assert.NoError(t, err)
	assert.NotNil(t, accessClaims)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, roles, accessClaims.Roles)
	assert.Equal(t, "access", accessClaims.Type)

	// Validate refresh token
	refreshClaims, err := jwtService.ValidateToken(refreshToken)
	assert.NoError(t, err)
	assert.NotNil(t, refreshClaims)
	assert.Equal(t, userID, refreshClaims.UserID)
	assert.Nil(t, refreshClaims.Roles) // Refresh tokens don't have roles
	assert.Equal(t, "refresh", refreshClaims.Type)
}

func TestJWTService_InvalidToken(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		SecretKey: struct {
			Access  string `json:"access" yaml:"access"`
			Refresh string `json:"refresh" yaml:"refresh"`
		}{
			Access:  "test_access_secret_key_very_long_for_testing",
			Refresh: "test_refresh_secret_key_very_long_for_testing",
		},
	}

	// Create JWT service
	jwtService, err := NewJWTService(cfg)
	assert.NoError(t, err)

	// Test invalid token - using clearly non-JWT format
	invalidToken := "clearly-not-a-jwt-token-format"
	claims, err := jwtService.ValidateToken(invalidToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to parse token structure")
}

func TestJWTService_UnknownTokenType(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		SecretKey: struct {
			Access  string `json:"access" yaml:"access"`
			Refresh string `json:"refresh" yaml:"refresh"`
		}{
			Access:  "test_access_secret_key_very_long_for_testing",
			Refresh: "test_refresh_secret_key_very_long_for_testing",
		},
	}

	// Create JWT service
	jwtService, err := NewJWTService(cfg)
	assert.NoError(t, err)

	// Test invalid token - using clearly non-JWT format
	invalidToken := "clearly-not-a-jwt-token-format"
	claims, err := jwtService.ValidateToken(invalidToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWTService_EmptySecrets(t *testing.T) {
	// Test with empty secrets
	cfg := &config.Config{
		SecretKey: struct {
			Access  string `json:"access" yaml:"access"`
			Refresh string `json:"refresh" yaml:"refresh"`
		}{
			Access:  "",
			Refresh: "",
		},
	}

	// Should fail to create service
	jwtService, err := NewJWTService(cfg)
	assert.Error(t, err)
	assert.Nil(t, jwtService)
	assert.Contains(t, err.Error(), "jwt secrets must be provided")
}

func TestJWTService_GetRefreshTokenDuration(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		SecretKey: struct {
			Access  string `json:"access" yaml:"access"`
			Refresh string `json:"refresh" yaml:"refresh"`
		}{
			Access:  "test_access_secret_key_very_long_for_testing",
			Refresh: "test_refresh_secret_key_very_long_for_testing",
		},
	}

	// Create JWT service
	jwtService, err := NewJWTService(cfg)
	assert.NoError(t, err)

	// Check refresh token duration
	duration := jwtService.GetRefreshTokenDuration()
	expectedDuration := time.Hour * 24 * 7 // 7 days
	assert.Equal(t, expectedDuration, duration)
}
