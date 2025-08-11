// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"golang.org/x/crypto/bcrypt"

	"radar/internal/domain/service"
)

// bcryptHasher is a concrete implementation of the PasswordHasher interface using bcrypt.
type bcryptHasher struct {
	// For bcrypt, the cost factor could be configurable here if needed.
	// cost int
}

// NewBcryptHasher is the constructor for bcryptHasher.
// It returns the implementation as a service.PasswordHasher interface.
func NewBcryptHasher() service.PasswordHasher {
	return &bcryptHasher{}
}

// Hash generates a salted hash from a plaintext password using bcrypt.
// bcrypt automatically handles salt generation.
func (h *bcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Check compares a plaintext password with a bcrypt hash.
func (h *bcryptHasher) Check(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	// err is nil if the password and hash match.
	return err == nil
}
