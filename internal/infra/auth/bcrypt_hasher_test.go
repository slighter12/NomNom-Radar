package auth

import (
	"testing"

	domainerrors "radar/internal/domain/errors"

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
	unicodePassword := "Pässphräse123!" //nolint:gosec // This is a test password, not a real password
	err = hasher.ValidatePasswordStrength(unicodePassword)
	assert.NoError(t, err) // Should be valid

	// Test password with only special characters
	specialOnlyPassword := "!@#$%^&*()"
	err = hasher.ValidatePasswordStrength(specialOnlyPassword)
	assert.Error(t, err) // Should fail because no letters or numbers
}

// TestBcryptHasher_ValidatePasswordLength tests the password length validation function
func TestBcryptHasher_ValidatePasswordLength(t *testing.T) {
	hasher := &bcryptHasher{cost: bcrypt.DefaultCost}
	config := hasher.getPasswordStrengthConfig()

	// Test valid lengths
	validPasswords := []string{
		"12345678",           // Exactly 8 characters
		"123456789",          // 9 characters
		"123456789012345678", // 18 characters
	}

	for _, password := range validPasswords {
		err := hasher.validatePasswordLength(password, config.MinLength)
		assert.NoError(t, err, "Expected no error for password with length %d: %s", len(password), password)
	}

	// Test invalid lengths
	invalidPasswords := []string{
		"",        // Empty string
		"1",       // 1 character
		"12",      // 2 characters
		"123",     // 3 characters
		"1234",    // 4 characters
		"12345",   // 5 characters
		"123456",  // 6 characters
		"1234567", // 7 characters
	}

	for _, password := range invalidPasswords {
		err := hasher.validatePasswordLength(password, config.MinLength)
		assert.Error(t, err, "Expected error for password with length %d: %s", len(password), password)
		assert.Contains(t, err.Error(), "must be at least 8 characters long")
	}
}

// TestBcryptHasher_ValidatePasswordCharacters tests the password character validation function
func TestBcryptHasher_ValidatePasswordCharacters(t *testing.T) {
	hasher := &bcryptHasher{cost: bcrypt.DefaultCost}
	config := hasher.getPasswordStrengthConfig()

	// Test valid character combinations
	validPasswords := []string{
		"StrongPass123!",   // All requirements met
		"MySecure@Pass1",   // All requirements met
		"Complex#Secret9",  // All requirements met
		"Valid$Phrase2024", // All requirements met
	}

	for _, password := range validPasswords {
		err := hasher.validatePasswordCharacters(password, config)
		assert.NoError(t, err, "Expected no error for valid password: %s", password)
	}

	// Test missing uppercase
	configNoUpper := config
	configNoUpper.RequireUppercase = false
	validNoUpper := "lowercase123!"
	err := hasher.validatePasswordCharacters(validNoUpper, configNoUpper)
	assert.NoError(t, err, "Expected no error for password without uppercase requirement")

	// Test missing lowercase
	configNoLower := config
	configNoLower.RequireLowercase = false
	validNoLower := "UPPERCASE123!"
	err = hasher.validatePasswordCharacters(validNoLower, configNoLower)
	assert.NoError(t, err, "Expected no error for password without lowercase requirement")

	// Test missing numbers
	configNoNumbers := config
	configNoNumbers.RequireNumbers = false
	validNoNumbers := "StrongPass!"
	err = hasher.validatePasswordCharacters(validNoNumbers, configNoNumbers)
	assert.NoError(t, err, "Expected no error for password without numbers requirement")

	// Test missing special characters
	configNoSpecial := config
	configNoSpecial.RequireSpecial = false
	validNoSpecial := "StrongPass123"
	err = hasher.validatePasswordCharacters(validNoSpecial, configNoSpecial)
	assert.NoError(t, err, "Expected no error for password without special characters requirement")

	// Test invalid passwords with specific missing requirements
	testCases := []struct {
		password    string
		expectedErr string
	}{
		{"password123!", "must contain at least one uppercase letter"},
		{"PASSWORD123!", "must contain at least one lowercase letter"},
		{"PasswordABC!", "must contain at least one number"},
		{"Password123", "must contain at least one special character"},
	}

	for _, tc := range testCases {
		err := hasher.validatePasswordCharacters(tc.password, config)
		assert.Error(t, err, "Expected error for password: %s", tc.password)
		assert.Contains(t, err.Error(), tc.expectedErr, "Error message should contain: %s", tc.expectedErr)
	}
}

// TestBcryptHasher_ValidatePasswordForbiddenWords tests the forbidden words validation function
func TestBcryptHasher_ValidatePasswordForbiddenWords(t *testing.T) {
	hasher := &bcryptHasher{cost: bcrypt.DefaultCost}
	config := hasher.getPasswordStrengthConfig()

	// Test valid passwords without forbidden words
	validPasswords := []string{
		"StrongPass123!",
		"MySecure@Pass1",
		"Complex#Secret9",
		"Valid$Phrase2024",
		"RandomString123!",
	}

	for _, password := range validPasswords {
		err := hasher.validatePasswordForbiddenWords(password, config.ForbiddenWords)
		assert.NoError(t, err, "Expected no error for valid password: %s", password)
	}

	// Test passwords with forbidden words
	forbiddenWords := []string{
		"password", "123456", "qwerty", "admin", "user",
		"login", "welcome", "test", "guest", "root",
	}

	for _, forbiddenWord := range forbiddenWords {
		// Test exact match
		err := hasher.validatePasswordForbiddenWords(forbiddenWord, config.ForbiddenWords)
		assert.Error(t, err, "Expected error for forbidden word: %s", forbiddenWord)
		assert.Contains(t, err.Error(), "contains forbidden words")

		// Test with additional characters
		passwordWithForbidden := forbiddenWord + "123!"
		err = hasher.validatePasswordForbiddenWords(passwordWithForbidden, config.ForbiddenWords)
		assert.Error(t, err, "Expected error for password containing forbidden word: %s", passwordWithForbidden)
		assert.Contains(t, err.Error(), "contains forbidden words")

		// Test case insensitive
		passwordUpper := "PASSWORD123!"
		err = hasher.validatePasswordForbiddenWords(passwordUpper, config.ForbiddenWords)
		assert.Error(t, err, "Expected error for password with uppercase forbidden word: %s", passwordUpper)
		assert.Contains(t, err.Error(), "contains forbidden words")
	}

	// Test empty forbidden words list
	err := hasher.validatePasswordForbiddenWords("AnyPassword123!", []string{})
	assert.NoError(t, err, "Expected no error when no forbidden words are configured")
}
