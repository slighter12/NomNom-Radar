// Package service defines interfaces for core, stateless domain logic.
// These services encapsulate business rules that don't naturally fit within a single entity.
package service

// PasswordHasher defines the interface for password hashing and verification.
// This abstracts the underlying hashing algorithm (e.g., bcrypt), keeping the domain pure.
type PasswordHasher interface {
	// Hash generates a salted hash from a plaintext password.
	Hash(password string) (string, error)

	// Check compares a plaintext password with a hash to see if they match.
	Check(password, hash string) bool
}
