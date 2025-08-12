// Package repository defines the interfaces for the persistence layer.
// These interfaces act as a contract between the domain/application layers and the infrastructure layer.
package repository

import (
	"context"
	"errors"

	"radar/internal/domain/entity"
)

// Domain-specific errors for authentication persistence.
// This allows the application layer to handle specific outcomes without depending on database-specific errors.
var (
	// ErrAuthNotFound is returned when an authentication method is not found.
	ErrAuthNotFound = errors.New("authentication method not found")
	// ErrTokenNotFound is returned when a refresh token is not found.
	ErrTokenNotFound = errors.New("refresh token not found")
)

// AuthRepository defines the interface for authentication-related database operations.
type AuthRepository interface {
	// CreateAuthentication persists a new authentication method (e.g., email/password, social login).
	CreateAuthentication(ctx context.Context, auth *entity.Authentication) error

	// FindAuthentication looks up an authentication record by provider and provider-specific user ID.
	// For example, when a user tries to log in with Google, we'd call this with
	// provider="google" and providerUserID=Google's 'sub' claim.
	FindAuthentication(ctx context.Context, provider entity.ProviderType, providerUserID string) (*entity.Authentication, error)

	// CreateRefreshToken persists a new refresh token, representing a user session.
	CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error

	// FindRefreshTokenByHash retrieves a refresh token record by its securely stored hash.
	FindRefreshTokenByHash(ctx context.Context, hash string) (*entity.RefreshToken, error)

	// DeleteRefreshTokenByHash deletes a refresh token by its hash, effectively ending a session.
	DeleteRefreshTokenByHash(ctx context.Context, hash string) error
}
