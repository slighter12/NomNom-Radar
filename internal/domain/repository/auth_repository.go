// Package repository defines the interfaces for the persistence layer.
// These interfaces act as a contract between the domain/application layers and the infrastructure layer.
package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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

	// FindAuthenticationByUserIDAndProvider finds an authentication method for a specific user and provider.
	// This is useful when checking if a user already has a specific authentication method linked.
	FindAuthenticationByUserIDAndProvider(ctx context.Context, userID uuid.UUID, provider entity.ProviderType) (*entity.Authentication, error)

	// UpdateAuthentication updates an existing authentication record.
	// This is useful for updating password hashes or other authentication details.
	UpdateAuthentication(ctx context.Context, auth *entity.Authentication) error

	// DeleteAuthentication removes an authentication method by its ID.
	// This ensures users can unlink authentication methods while maintaining at least one.
	DeleteAuthentication(ctx context.Context, id uuid.UUID) error

	// ListAuthenticationsByUserID returns all authentication methods for a specific user.
	// This allows users to see and manage their linked authentication methods.
	ListAuthenticationsByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Authentication, error)
}
