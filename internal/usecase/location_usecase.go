package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// AddLocationInput represents the input for adding a new location
type AddLocationInput struct {
	Label       string  `json:"label"`
	FullAddress string  `json:"full_address"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	IsPrimary   bool    `json:"is_primary"`
	IsActive    bool    `json:"is_active"`
}

// UpdateLocationInput represents the input for updating an existing location
type UpdateLocationInput struct {
	Label       *string  `json:"label,omitempty"`
	FullAddress *string  `json:"full_address,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	IsPrimary   *bool    `json:"is_primary,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// LocationUsecase defines the interface for location management use cases
type LocationUsecase interface {
	// User location management
	GetUserLocations(ctx context.Context, userID uuid.UUID) ([]*entity.Address, error)
	AddUserLocation(ctx context.Context, userID uuid.UUID, input *AddLocationInput) (*entity.Address, error)
	UpdateUserLocation(ctx context.Context, userID, locationID uuid.UUID, input *UpdateLocationInput) (*entity.Address, error)
	DeleteUserLocation(ctx context.Context, userID, locationID uuid.UUID) error

	// Merchant location management
	GetMerchantLocations(ctx context.Context, merchantID uuid.UUID) ([]*entity.Address, error)
	AddMerchantLocation(ctx context.Context, merchantID uuid.UUID, input *AddLocationInput) (*entity.Address, error)
	UpdateMerchantLocation(ctx context.Context, merchantID, locationID uuid.UUID, input *UpdateLocationInput) (*entity.Address, error)
	DeleteMerchantLocation(ctx context.Context, merchantID, locationID uuid.UUID) error
}
