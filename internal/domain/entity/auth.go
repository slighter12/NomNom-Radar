// Package entity contains the core business objects of the project,
// each representing a unique, identifiable concept within the domain.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// Authentication represents a single method of logging in (a credential).
// For example, a user's email/password is one record, while a linked Google account is another.
type Authentication struct {
	ID             uuid.UUID // The unique ID for this specific authentication record itself.
	UserID         uuid.UUID // Links this authentication method to the User it belongs to.
	Provider       string    // The authentication provider, e.g., "email", "google", "apple".
	ProviderUserID string    // The user's unique ID from the external provider (e.g., Google's 'sub' claim).
	PasswordHash   string    // Stores the bcrypt-hashed password, only used when the Provider is "email".
	CreatedAt      time.Time // Timestamp of when this authentication method was linked to the user account.
}

// RefreshToken represents a long-lived, authorized user session.
// It is used to obtain a new Access Token after the old one expires, without requiring credentials.
type RefreshToken struct {
	ID        uuid.UUID // The unique ID for this specific refresh token record.
	UserID    uuid.UUID // Links this session to the User it belongs to.
	TokenHash string    // Stores a SHA-256 hash of the raw refresh token for secure comparison in the database.
	ExpiresAt time.Time // The exact time when this refresh token will expire and become invalid.
	CreatedAt time.Time // Timestamp of when this session was created (i.e., when the user logged in).
}
