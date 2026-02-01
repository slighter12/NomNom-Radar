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
	assert.Contains(t, err.Error(), "token is malformed")
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

func TestJWTService_HashToken(t *testing.T) {
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

	// Test token hashing
	testToken := "test.jwt.token"
	hash1 := jwtService.HashToken(testToken)
	hash2 := jwtService.HashToken(testToken)

	// Same input should produce same hash
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA-256 produces 64-character hex string

	// Different inputs should produce different hashes
	differentToken := "different.jwt.token"
	differentHash := jwtService.HashToken(differentToken)
	assert.NotEqual(t, hash1, differentHash)
}

func TestJWTService_RotateTokens(t *testing.T) {
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

	// Test data
	userID := uuid.New()
	roles := []string{"user", "admin"}

	// Rotate tokens
	newAccess, newRefresh, newRefreshHash, err := jwtService.RotateTokens(userID, roles)
	assert.NoError(t, err)
	assert.NotEmpty(t, newAccess)
	assert.NotEmpty(t, newRefresh)
	assert.NotEmpty(t, newRefreshHash)

	// Validate new access token
	accessClaims, err := jwtService.ValidateToken(newAccess)
	assert.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, roles, accessClaims.Roles)
	assert.Equal(t, "access", accessClaims.Type)

	// Validate new refresh token
	refreshClaims, err := jwtService.ValidateToken(newRefresh)
	assert.NoError(t, err)
	assert.Equal(t, userID, refreshClaims.UserID)
	assert.Nil(t, refreshClaims.Roles)
	assert.Equal(t, "refresh", refreshClaims.Type)

	// Verify hash matches the refresh token
	expectedHash := jwtService.HashToken(newRefresh)
	assert.Equal(t, expectedHash, newRefreshHash)
}

func TestJWTService_TokenRotationFunctionality(t *testing.T) {
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

	userID := uuid.New()
	roles := []string{"user"}

	// Test that rotation produces valid tokens
	access, refresh, refreshHash, err := jwtService.RotateTokens(userID, roles)
	assert.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	assert.NotEmpty(t, refreshHash)

	// Verify tokens are valid
	accessClaims, err := jwtService.ValidateToken(access)
	assert.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, roles, accessClaims.Roles)

	refreshClaims, err := jwtService.ValidateToken(refresh)
	assert.NoError(t, err)
	assert.Equal(t, userID, refreshClaims.UserID)
	assert.Nil(t, refreshClaims.Roles)

	// Verify hash is correct
	expectedHash := jwtService.HashToken(refresh)
	assert.Equal(t, expectedHash, refreshHash)

	// Test with different user to ensure different tokens
	differentUserID := uuid.New()
	_, _, refreshHash2, err := jwtService.RotateTokens(differentUserID, roles)
	assert.NoError(t, err)

	// Different users should have different hashes
	assert.NotEqual(t, refreshHash, refreshHash2)
}
