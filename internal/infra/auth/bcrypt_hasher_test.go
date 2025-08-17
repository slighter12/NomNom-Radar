package auth

import (
	domainerrors "radar/internal/domain/errors"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasher_Hash(t *testing.T) {
	hasher := NewBcryptHasher()

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
	hasher := NewBcryptHasher()

	// Test weak passwords that should fail validation
	weakPasswords := []string{
		"123",         // Too short
		"password",    // Forbidden word
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
	hasher := NewBcryptHasher()
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
	hasher := NewBcryptHasher()

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
		{"Password123!", "contains forbidden words"},
		{"MyAdmin123!", "contains forbidden words"},
	}

	for _, tc := range testCases {
		err := hasher.ValidatePasswordStrength(tc.password)
		assert.Error(t, err, "Expected error for password: %s", tc.password)
		assert.Contains(t, err.Error(), tc.expectedErr, "Error message should contain: %s", tc.expectedErr)
	}
}

func TestBcryptHasher_WithCustomCost(t *testing.T) {
	customCost := 6 // Lower cost for faster testing
	hasher := NewBcryptHasherWithCost(customCost)

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

func TestBcryptHasher_PasswordStrengthHelpers(t *testing.T) {
	hasher := &bcryptHasher{}

	// Test hasUppercase
	assert.True(t, hasher.hasUppercase("Password"))
	assert.False(t, hasher.hasUppercase("password"))

	// Test hasLowercase
	assert.True(t, hasher.hasLowercase("Password"))
	assert.False(t, hasher.hasLowercase("PASSWORD"))

	// Test hasNumbers
	assert.True(t, hasher.hasNumbers("Password123"))
	assert.False(t, hasher.hasNumbers("Password"))

	// Test hasSpecialChars
	assert.True(t, hasher.hasSpecialChars("Password!"))
	assert.False(t, hasher.hasSpecialChars("Password"))

	// Test containsForbiddenWords
	forbiddenWords := []string{"password", "admin"}
	assert.True(t, hasher.containsForbiddenWords("MyPassword123", forbiddenWords))
	assert.True(t, hasher.containsForbiddenWords("AdminUser", forbiddenWords))
	assert.False(t, hasher.containsForbiddenWords("SecurePass123", forbiddenWords))
}

func TestBcryptHasher_EdgeCases(t *testing.T) {
	hasher := NewBcryptHasher()

	// Test empty password
	err := hasher.ValidatePasswordStrength("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 8 characters long")

	// Test forbidden words
	longPassword := "VeryLongPassword123!" + string(make([]byte, 1000))
	err = hasher.ValidatePasswordStrength(longPassword)
	assert.True(t, errors.Is(err, domainerrors.ErrPasswordForbiddenWords)) // Should be valid if it meets all requirements

	// Test password with unicode characters
	unicodePassword := "Pässphräse123!"
	err = hasher.ValidatePasswordStrength(unicodePassword)
	assert.NoError(t, err) // Should be valid

	// Test password with only special characters
	specialOnlyPassword := "!@#$%^&*()"
	err = hasher.ValidatePasswordStrength(specialOnlyPassword)
	assert.Error(t, err) // Should fail because no letters or numbers
}
