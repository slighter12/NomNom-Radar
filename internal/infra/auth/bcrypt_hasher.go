// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/service"

	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// bcryptHasher is a concrete implementation of the PasswordHasher interface using bcrypt.
type bcryptHasher struct {
	// cost is the bcrypt cost factor for hashing
	cost int
}

// PasswordStrengthConfig defines password strength requirements
type PasswordStrengthConfig struct {
	MinLength        int
	RequireUppercase bool
	RequireLowercase bool
	RequireNumbers   bool
	RequireSpecial   bool
	ForbiddenWords   []string
}

// NewBcryptHasher is the constructor for bcryptHasher.
// It returns the implementation as a service.PasswordHasher interface.
func NewBcryptHasher() service.PasswordHasher {
	return &bcryptHasher{
		cost: bcrypt.DefaultCost, // Use bcrypt default cost (10)
	}
}

// NewBcryptHasherWithCost creates a bcrypt hasher with custom cost factor.
// Higher cost means more secure but slower hashing.
func NewBcryptHasherWithCost(cost int) service.PasswordHasher {
	return &bcryptHasher{
		cost: cost,
	}
}

// Hash generates a salted hash from a plaintext password using bcrypt.
// bcrypt automatically handles salt generation.
func (h *bcryptHasher) Hash(password string) (string, error) {
	// Validate password strength before hashing
	if err := h.ValidatePasswordStrength(password); err != nil {
		return "", errors.Wrap(err, "password does not meet strength requirements")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", errors.Wrap(err, "failed to hash password")
	}

	return string(bytes), nil
}

// Check compares a plaintext password with a bcrypt hash.
func (h *bcryptHasher) Check(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	// err is nil if the password and hash match.
	return err == nil
}

// ValidatePasswordStrength validates that a password meets security requirements.
func (h *bcryptHasher) ValidatePasswordStrength(password string) error {
	config := h.getPasswordStrengthConfig()

	// Check minimum length
	if len(password) < config.MinLength {
		return fmt.Errorf("password must be at least %d characters long", config.MinLength)
	}

	// Check for uppercase letters
	if config.RequireUppercase && !h.hasUppercase(password) {
		return errors.New("password must contain at least one uppercase letter")
	}

	// Check for lowercase letters
	if config.RequireLowercase && !h.hasLowercase(password) {
		return errors.New("password must contain at least one lowercase letter")
	}

	// Check for numbers
	if config.RequireNumbers && !h.hasNumbers(password) {
		return errors.New("password must contain at least one number")
	}

	// Check for special characters
	if config.RequireSpecial && !h.hasSpecialChars(password) {
		return errors.New("password must contain at least one special character")
	}

	// Check for forbidden words
	if h.containsForbiddenWords(password, config.ForbiddenWords) {
		return domainerrors.ErrPasswordForbiddenWords.WrapMessage("password contains forbidden words or patterns")
	}

	return nil
}

// getPasswordStrengthConfig returns the default password strength configuration
func (h *bcryptHasher) getPasswordStrengthConfig() PasswordStrengthConfig {
	return PasswordStrengthConfig{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSpecial:   true,
		ForbiddenWords: []string{
			"password", "123456", "qwerty", "admin", "user",
			"login", "welcome", "test", "guest", "root",
		},
	}
}

// hasUppercase checks if password contains uppercase letters
func (h *bcryptHasher) hasUppercase(password string) bool {
	for _, char := range password {
		if unicode.IsUpper(char) {
			return true
		}
	}
	return false
}

// hasLowercase checks if password contains lowercase letters
func (h *bcryptHasher) hasLowercase(password string) bool {
	for _, char := range password {
		if unicode.IsLower(char) {
			return true
		}
	}
	return false
}

// hasNumbers checks if password contains numbers
func (h *bcryptHasher) hasNumbers(password string) bool {
	for _, char := range password {
		if unicode.IsDigit(char) {
			return true
		}
	}
	return false
}

// hasSpecialChars checks if password contains special characters
func (h *bcryptHasher) hasSpecialChars(password string) bool {
	// Define special characters pattern
	specialChars := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?~` + "`" + `]`)
	return specialChars.MatchString(password)
}

// containsForbiddenWords checks if password contains any forbidden words
func (h *bcryptHasher) containsForbiddenWords(password string, forbiddenWords []string) bool {
	lowerPassword := strings.ToLower(password)
	for _, word := range forbiddenWords {
		if strings.Contains(lowerPassword, strings.ToLower(word)) {
			return true
		}
	}
	return false
}
