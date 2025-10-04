package impl

import (
	"context"
	"errors"
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
	return &locationService{
		addressRepo: addressRepo,
		config:      cfg,
	}
}

// GetUserLocations retrieves all locations for a user
func (s *locationService) GetUserLocations(ctx context.Context, userID uuid.UUID) ([]*entity.Address, error) {
	return s.addressRepo.FindAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile)
}

// AddUserLocation adds a new location for a user
func (s *locationService) AddUserLocation(ctx context.Context, userID uuid.UUID, input *usecase.AddLocationInput) (*entity.Address, error) {
	// Check location limit
	count, err := s.addressRepo.CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile)
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
	}

	// Verify ownership
	if address.OwnerID != userID || address.OwnerType != entity.OwnerTypeUserProfile {
		return nil, ErrUnauthorized
	}

	// Update fields if provided
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

	if err := s.addressRepo.UpdateAddress(ctx, address); err != nil {
		return nil, err
	}

	return address, nil
}

// DeleteUserLocation deletes a location for a user (soft delete)
func (s *locationService) DeleteUserLocation(ctx context.Context, userID, locationID uuid.UUID) error {
	// Fetch existing address to verify ownership
	address, err := s.addressRepo.FindAddressByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, repository.ErrAddressNotFound) {
			return ErrLocationNotFound
		}
		return err
	}

	// Verify ownership
	if address.OwnerID != userID || address.OwnerType != entity.OwnerTypeUserProfile {
		return ErrUnauthorized
	}

	return s.addressRepo.DeleteAddress(ctx, locationID)
}

// GetMerchantLocations retrieves all locations for a merchant
func (s *locationService) GetMerchantLocations(ctx context.Context, merchantID uuid.UUID) ([]*entity.Address, error) {
	return s.addressRepo.FindAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile)
}

// AddMerchantLocation adds a new location for a merchant
func (s *locationService) AddMerchantLocation(ctx context.Context, merchantID uuid.UUID, input *usecase.AddLocationInput) (*entity.Address, error) {
	// Check location limit
	count, err := s.addressRepo.CountAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile)
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
	}

	// Verify ownership
	if address.OwnerID != merchantID || address.OwnerType != entity.OwnerTypeMerchantProfile {
		return nil, ErrUnauthorized
	}

	// Update fields if provided
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

	if err := s.addressRepo.UpdateAddress(ctx, address); err != nil {
		return nil, err
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
		return err
	}

	// Verify ownership
	if address.OwnerID != merchantID || address.OwnerType != entity.OwnerTypeMerchantProfile {
		return ErrUnauthorized
	}

	return s.addressRepo.DeleteAddress(ctx, locationID)
}
