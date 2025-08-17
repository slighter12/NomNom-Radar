// Package repository defines the interfaces for the persistence layer.
// These interfaces act as a contract between the domain/application layers and the infrastructure layer.
package repository

import (
	"context"
	"errors"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// ErrUserNotFound is a domain-specific error returned when a user is not found.
var ErrUserNotFound = errors.New("user not found")

// UserRepository defines the standard operations for user persistence.
// The application layer will depend on this interface, not the concrete implementation.
type UserRepository interface {
	// FindByID retrieves a single user by their unique ID.
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)

	// FindByEmail retrieves a single user by their email address.
	FindByEmail(ctx context.Context, email string) (*entity.User, error)

	// Create persists a new user entity to the storage.
	Create(ctx context.Context, user *entity.User) error

	// Update modifies an existing user entity in the storage.
	Update(ctx context.Context, user *entity.User) error

	// Note: Delete method can be added here as needed.
}
