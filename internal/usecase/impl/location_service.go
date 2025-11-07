package impl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
)

var (
	// ErrLocationLimitReached is returned when the location limit is reached
	ErrLocationLimitReached = errors.New("location limit reached")
	// ErrLocationNotFound is returned when a location is not found
	ErrLocationNotFound = errors.New("location not found")
	// ErrUnauthorized is returned when a user tries to access a location they don't own
	ErrUnauthorized = errors.New("unauthorized to access this location")
)

type locationService struct {
	addressRepo repository.AddressRepository
	config      *config.Config
}

// NewLocationService creates a new location service instance
func NewLocationService(addressRepo repository.AddressRepository, cfg *config.Config) usecase.LocationUsecase {
	// If LocationNotification is not configured, provide a default configuration
	if cfg.LocationNotification == nil {
		cfg.LocationNotification = &config.LocationNotificationConfig{
			UserMaxLocations:     5,    // Default to 5 locations
			MerchantMaxLocations: 10,   // Default to 10 locations
			DefaultRadius:        1000, // Default to 1km
			MaxRadius:            5000, // Default to 5km
		}
	}

	return &locationService{
		addressRepo: addressRepo,
		config:      cfg,
	}
}

// GetUserLocations retrieves all locations for a user
func (s *locationService) GetUserLocations(ctx context.Context, userID uuid.UUID) ([]*entity.Address, error) {
	addresses, err := s.addressRepo.FindAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to find addresses by owner: %w", err)
	}

	return addresses, nil
}

// AddUserLocation adds a new location for a user
func (s *locationService) AddUserLocation(ctx context.Context, userID uuid.UUID, input *usecase.AddLocationInput) (*entity.Address, error) {
	// Check location limit
	count, err := s.addressRepo.CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to count addresses by owner: %w", err)
	}

	maxLocations := s.config.LocationNotification.UserMaxLocations
	if count >= int64(maxLocations) {
		return nil, ErrLocationLimitReached
	}

	// Create new address
	address := &entity.Address{
		ID:          uuid.New(),
		OwnerID:     userID,
		OwnerType:   entity.OwnerTypeUserProfile,
		Label:       input.Label,
		FullAddress: input.FullAddress,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		IsPrimary:   input.IsPrimary,
		IsActive:    input.IsActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.addressRepo.CreateAddress(ctx, address); err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}

	return address, nil
}

// UpdateUserLocation updates an existing location for a user
func (s *locationService) UpdateUserLocation(ctx context.Context, userID, locationID uuid.UUID, input *usecase.UpdateLocationInput) (*entity.Address, error) {
	// Fetch existing address
	address, err := s.addressRepo.FindAddressByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, repository.ErrAddressNotFound) {
			return nil, ErrLocationNotFound
		}

		return nil, fmt.Errorf("failed to find address by ID: %w", err)
	}

	// Verify ownership
	if address.OwnerID != userID || address.OwnerType != entity.OwnerTypeUserProfile {
		return nil, ErrUnauthorized
	}

	// Apply updates
	s.applyAddressUpdates(address, input)

	if err := s.addressRepo.UpdateAddress(ctx, address); err != nil {
		return nil, fmt.Errorf("failed to update address: %w", err)
	}

	return address, nil
}

// applyAddressUpdates applies the update input to an address
func (s *locationService) applyAddressUpdates(address *entity.Address, input *usecase.UpdateLocationInput) {
	if input.Label != nil {
		address.Label = *input.Label
	}
	if input.FullAddress != nil {
		address.FullAddress = *input.FullAddress
	}
	if input.Latitude != nil {
		address.Latitude = *input.Latitude
	}
	if input.Longitude != nil {
		address.Longitude = *input.Longitude
	}
	if input.IsPrimary != nil {
		address.IsPrimary = *input.IsPrimary
	}
	if input.IsActive != nil {
		address.IsActive = *input.IsActive
	}
	address.UpdatedAt = time.Now()
}

// DeleteUserLocation deletes a location for a user (soft delete)
func (s *locationService) DeleteUserLocation(ctx context.Context, userID, locationID uuid.UUID) error {
	// Fetch existing address to verify ownership
	address, err := s.addressRepo.FindAddressByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, repository.ErrAddressNotFound) {
			return ErrLocationNotFound
		}

		return fmt.Errorf("failed to find address by ID: %w", err)
	}

	// Verify ownership
	if address.OwnerID != userID || address.OwnerType != entity.OwnerTypeUserProfile {
		return ErrUnauthorized
	}

	if err := s.addressRepo.DeleteAddress(ctx, locationID); err != nil {
		return fmt.Errorf("failed to delete address: %w", err)
	}

	return nil
}

// GetMerchantLocations retrieves all locations for a merchant
func (s *locationService) GetMerchantLocations(ctx context.Context, merchantID uuid.UUID) ([]*entity.Address, error) {
	addresses, err := s.addressRepo.FindAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to find addresses by owner: %w", err)
	}

	return addresses, nil
}

// AddMerchantLocation adds a new location for a merchant
func (s *locationService) AddMerchantLocation(ctx context.Context, merchantID uuid.UUID, input *usecase.AddLocationInput) (*entity.Address, error) {
	// Check location limit
	count, err := s.addressRepo.CountAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to count addresses by owner: %w", err)
	}

	maxLocations := s.config.LocationNotification.MerchantMaxLocations
	if count >= int64(maxLocations) {
		return nil, ErrLocationLimitReached
	}

	// Create new address
	address := &entity.Address{
		ID:          uuid.New(),
		OwnerID:     merchantID,
		OwnerType:   entity.OwnerTypeMerchantProfile,
		Label:       input.Label,
		FullAddress: input.FullAddress,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		IsPrimary:   input.IsPrimary,
		IsActive:    input.IsActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.addressRepo.CreateAddress(ctx, address); err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}

	return address, nil
}

// UpdateMerchantLocation updates an existing location for a merchant
func (s *locationService) UpdateMerchantLocation(ctx context.Context, merchantID, locationID uuid.UUID, input *usecase.UpdateLocationInput) (*entity.Address, error) {
	// Fetch existing address
	address, err := s.addressRepo.FindAddressByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, repository.ErrAddressNotFound) {
			return nil, ErrLocationNotFound
		}

		return nil, fmt.Errorf("failed to find address by ID: %w", err)
	}

	// Verify ownership
	if address.OwnerID != merchantID || address.OwnerType != entity.OwnerTypeMerchantProfile {
		return nil, ErrUnauthorized
	}

	// Apply updates
	s.applyAddressUpdates(address, input)

	if err := s.addressRepo.UpdateAddress(ctx, address); err != nil {
		return nil, fmt.Errorf("failed to update address: %w", err)
	}

	return address, nil
}

// DeleteMerchantLocation deletes a location for a merchant (soft delete)
func (s *locationService) DeleteMerchantLocation(ctx context.Context, merchantID, locationID uuid.UUID) error {
	// Fetch existing address to verify ownership
	address, err := s.addressRepo.FindAddressByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, repository.ErrAddressNotFound) {
			return ErrLocationNotFound
		}

		return fmt.Errorf("failed to find address by ID: %w", err)
	}

	// Verify ownership
	if address.OwnerID != merchantID || address.OwnerType != entity.OwnerTypeMerchantProfile {
		return ErrUnauthorized
	}

	if err := s.addressRepo.DeleteAddress(ctx, locationID); err != nil {
		return fmt.Errorf("failed to delete address: %w", err)
	}

	return nil
}
