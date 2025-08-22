package auth

import (
	"testing"

	"radar/config"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasher_Hash(t *testing.T) {
	cfg := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}
	hasher, err := NewBcryptHasher(0, cfg)
	assert.NoError(t, err)

	// Test valid strong password
	strongPassword := "StrongPass123!"
	hash, err := hasher.Hash(strongPassword)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, strongPassword, hash)

	// Verify the hash can be checked
	assert.True(t, hasher.Check(strongPassword, hash))
}

func TestBcryptHasher_HashWithWeakPassword(t *testing.T) {
	cfg := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}
	hasher, err := NewBcryptHasher(0, cfg)
	assert.NoError(t, err)

	// Test weak passwords that should fail validation
	weakPasswords := []string{
		"123",         // Too short
		"PASSWORD123", // No lowercase
		"password123", // No uppercase
		"PasswordABC", // No numbers
		"Password123", // No special characters
	}

	for _, weakPassword := range weakPasswords {
		_, err := hasher.Hash(weakPassword)
		assert.Error(t, err, "Expected error for weak password: %s", weakPassword)
	}
}

func TestBcryptHasher_Check(t *testing.T) {
	cfg := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}
	hasher, err := NewBcryptHasher(0, cfg)
	assert.NoError(t, err)
	password := "StrongPass123!"

	// Generate hash
	hash, err := hasher.Hash(password)
	assert.NoError(t, err)

	// Test correct password
	assert.True(t, hasher.Check(password, hash))

	// Test incorrect password
	assert.False(t, hasher.Check("WrongPassword123!", hash))

	// Test empty password
	assert.False(t, hasher.Check("", hash))

	// Test with invalid hash
	assert.False(t, hasher.Check(password, "invalid_hash"))
}

func TestBcryptHasher_ValidatePasswordStrength(t *testing.T) {
	cfg := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}
	hasher, err := NewBcryptHasher(0, cfg)
	assert.NoError(t, err)

	// Test valid passwords
	validPasswords := []string{
		"StrongPass123!",
		"MySecure@Pass1",
		"Complex#Secret9",
		"Valid$Phrase2024",
	}

	for _, password := range validPasswords {
		err := hasher.ValidatePasswordStrength(password)
		assert.NoError(t, err, "Expected no error for valid password: %s", password)
	}

	// Test invalid passwords with specific error cases
	testCases := []struct {
		password    string
		expectedErr string
	}{
		{"123", "must be at least 8 characters long"},
		{"PASSWORD123!", "must contain at least one lowercase letter"},
		{"password123!", "must contain at least one uppercase letter"},
		{"PasswordABC!", "must contain at least one number"},
		{"Password123", "must contain at least one special character"},
	}

	for _, tc := range testCases {
		err := hasher.ValidatePasswordStrength(tc.password)
		assert.Error(t, err, "Expected error for password: %s", tc.password)
		assert.Contains(t, err.Error(), tc.expectedErr, "Error message should contain: %s", tc.expectedErr)
	}
}

func TestBcryptHasher_WithCustomCost(t *testing.T) {
	customCost := 6 // Lower cost for faster testing
	cfg := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}
	hasher, err := NewBcryptHasher(customCost, cfg)
	assert.NoError(t, err)

	password := "StrongPass123!"
	hash, err := hasher.Hash(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify the hash uses the correct cost
	cost, err := bcrypt.Cost([]byte(hash))
	assert.NoError(t, err)
	assert.Equal(t, customCost, cost)

	// Verify the hash can be checked
	assert.True(t, hasher.Check(password, hash))
}

func TestBcryptHasher_EdgeCases(t *testing.T) {
	cfg := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}
	hasher, err := NewBcryptHasher(0, cfg)
	assert.NoError(t, err)

	// Test empty password
	err = hasher.ValidatePasswordStrength("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 8 characters long")

	// Test password with unicode characters
	unicodePassword := "Pässphräse123!"
	err = hasher.ValidatePasswordStrength(unicodePassword)
	assert.NoError(t, err) // Should be valid

	// Test password with only special characters
	specialOnlyPassword := "!@#$%^&*()"
	err = hasher.ValidatePasswordStrength(specialOnlyPassword)
	assert.Error(t, err) // Should fail because no letters or numbers
}

// TestBcryptHasher_WithCustomConfig tests the hasher with custom password strength configuration
func TestBcryptHasher_WithCustomConfig(t *testing.T) {
	customConfig := &config.Config{
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        10,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   false, // Disable special character requirement
			MaxLength:        50,
		},
	}

	hasher, err := NewBcryptHasher(0, customConfig)
	assert.NoError(t, err)

	// Test password that meets custom requirements
	validPassword := "StrongPass123" // 13 chars, no special chars required
	err = hasher.ValidatePasswordStrength(validPassword)
	assert.NoError(t, err, "Expected no error for password meeting custom requirements")

	// Test password that's too short for custom config
	shortPassword := "Pass123" // Only 7 chars, but min is 10
	err = hasher.ValidatePasswordStrength(shortPassword)
	assert.Error(t, err, "Expected error for password too short for custom config")
	assert.Contains(t, err.Error(), "must be at least 10 characters long")

	// Test password that's too long for custom config
	longPassword := "ThisIsAVeryLongPasswordThatExceedsTheMaximumLengthLimit123"
	err = hasher.ValidatePasswordStrength(longPassword)
	assert.Error(t, err, "Expected error for password too long for custom config")
	assert.Contains(t, err.Error(), "must be no more than 50 characters long")
}
