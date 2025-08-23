// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"fmt"
	"regexp"
	"unicode"

	"radar/config"
	"radar/internal/domain/service"

	"golang.org/x/crypto/bcrypt"
)

// bcryptHasher is a concrete implementation of the PasswordHasher interface using bcrypt.
type bcryptHasher struct {
	// bcryptCost holds the bcrypt cost factor for hashing
	bcryptCost int
	// passwordStrengthConfig holds the password strength configuration
	passwordStrengthConfig *config.PasswordStrengthConfig
	specialChars           *regexp.Regexp
}

// NewBcryptHasher creates a bcrypt hasher with custom configuration.
func NewBcryptHasher(cfg *config.Config) (service.PasswordHasher, error) {
	// Get bcrypt cost from config, use default if not specified
	bcryptCost := bcrypt.DefaultCost
	if cfg.Auth != nil && cfg.Auth.BcryptCost > 0 {
		bcryptCost = cfg.Auth.BcryptCost
	}

	return &bcryptHasher{
		bcryptCost:             bcryptCost,
		passwordStrengthConfig: cfg.PasswordStrength,
		specialChars:           regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?~` + "`" + `]`),
	}, nil
}

// Hash generates a salted hash from a plaintext password using bcrypt.
// bcrypt automatically handles salt generation.
func (h *bcryptHasher) Hash(password string) (string, error) {
	// Validate password strength before hashing
	if err := h.ValidatePasswordStrength(password); err != nil {
		return "", fmt.Errorf("password does not meet strength requirements: %w", err)
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
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
	// Validate all password requirements
	if err := h.validatePasswordLength(password, h.passwordStrengthConfig.MinLength, h.passwordStrengthConfig.MaxLength); err != nil {
		return err
	}

	if err := h.validatePasswordCharacters(password); err != nil {
		return err
	}

	return nil
}

// validatePasswordLength checks if password meets length requirements
func (h *bcryptHasher) validatePasswordLength(password string, minLength, maxLength int) error {
	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters long", minLength)
	}

	if maxLength > 0 && len(password) > maxLength {
		return fmt.Errorf("password must be no more than %d characters long", maxLength)
	}

	return nil
}

// validatePasswordCharacters checks if password contains required character types
func (h *bcryptHasher) validatePasswordCharacters(password string) error {
	if h.passwordStrengthConfig.RequireUppercase && !h.hasUppercase(password) {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}

	if h.passwordStrengthConfig.RequireLowercase && !h.hasLowercase(password) {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}

	if h.passwordStrengthConfig.RequireNumbers && !h.hasNumbers(password) {
		return fmt.Errorf("password must contain at least one number")
	}

	if h.passwordStrengthConfig.RequireSpecial && !h.hasSpecialChars(password) {
		return fmt.Errorf("password must contain at least one special character")
	}

	return nil
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
	return h.specialChars.MatchString(password)
}
