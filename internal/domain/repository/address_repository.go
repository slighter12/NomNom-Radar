// Package repository defines the interfaces for the persistence layer.
// These interfaces act as a contract between the domain/application layers and the infrastructure layer.
package repository

import (
	"context"

	"radar/internal/domain/entity"
	"radar/internal/errors"

	"github.com/google/uuid"
)

// Domain-specific errors for address persistence.
var (
	// ErrAddressNotFound is returned when an address is not found.
	ErrAddressNotFound = errors.New("address not found")
	// ErrPrimaryAddressConflict is returned when trying to set multiple primary addresses for the same owner.
	ErrPrimaryAddressConflict = errors.New("owner already has a primary address")
)

// AddressRepository defines the interface for address-related database operations.
// It supports polymorphic associations where addresses can belong to different owner types.
type AddressRepository interface {
	// CreateAddress persists a new address for an owner.
	CreateAddress(ctx context.Context, address *entity.Address) error

	// FindAddressByID retrieves an address by its unique ID.
	FindAddressByID(ctx context.Context, id uuid.UUID) (*entity.Address, error)

	// FindAddressesByOwner retrieves all addresses for a specific owner (user_profile, merchant_profile, etc.).
	FindAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) ([]*entity.Address, error)

	// FindPrimaryAddressByOwner retrieves the primary address for a specific owner.
	// Returns ErrAddressNotFound if no primary address exists.
	FindPrimaryAddressByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) (*entity.Address, error)

	// UpdateAddress updates an existing address record.
	UpdateAddress(ctx context.Context, address *entity.Address) error

	// DeleteAddress removes an address by its ID.
	DeleteAddress(ctx context.Context, id uuid.UUID) error

	// CountAddressesByOwner returns the total count of addresses for a specific owner.
	// Used for checking location limits.
	CountAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) (int64, error)

	// FindActiveAddressesByOwner retrieves all active addresses (IsActive=true and not soft-deleted) for a specific owner.
	FindActiveAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) ([]*entity.Address, error)
}
