package impl

import (
	"context"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

var (
	// ErrLocationLimitReached is returned when the location limit is reached
	ErrLocationLimitReached = domainerrors.ErrLocationLimitReached
	// ErrLocationNotFound is returned when a location is not found
	ErrLocationNotFound = domainerrors.ErrAddressNotFound
	// ErrUnauthorized is returned when a user tries to access a location they don't own
	ErrUnauthorized = domainerrors.ErrAddressOwnershipViolation
)

type locationService struct {
	addressRepo repository.AddressRepository
	config      *config.Config
}

// LocationServiceParams holds dependencies for LocationService, injected by Fx.
type LocationServiceParams struct {
	fx.In

	AddressRepo repository.AddressRepository
	Config      *config.Config
}

// NewLocationService creates a new location service instance
func NewLocationService(params LocationServiceParams) usecase.LocationUsecase {
	// If LocationNotification is not configured, provide a default configuration
	if params.Config.LocationNotification == nil {
		params.Config.LocationNotification = &config.LocationNotificationConfig{
			UserMaxLocations:     5,    // Default to 5 locations
			MerchantMaxLocations: 10,   // Default to 10 locations
			DefaultRadius:        1000, // Default to 1km
			MaxRadius:            5000, // Default to 5km
		}
	}

	return &locationService{
		addressRepo: params.AddressRepo,
		config:      params.Config,
	}
}

// GetUserLocations retrieves all locations for a user
func (s *locationService) GetUserLocations(ctx context.Context, userID uuid.UUID) ([]*entity.Address, error) {
	return s.getLocations(ctx, userID, entity.OwnerTypeUserProfile)
}

// AddUserLocation adds a new location for a user
func (s *locationService) AddUserLocation(ctx context.Context, userID uuid.UUID, input *usecase.AddLocationInput) (*entity.Address, error) {
	return s.addLocation(ctx, userID, entity.OwnerTypeUserProfile, s.config.LocationNotification.UserMaxLocations, input)
}

// UpdateUserLocation updates an existing location for a user
func (s *locationService) UpdateUserLocation(ctx context.Context, userID, locationID uuid.UUID, input *usecase.UpdateLocationInput) (*entity.Address, error) {
	return s.updateLocation(ctx, userID, locationID, entity.OwnerTypeUserProfile, input)
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
	return s.deleteLocation(ctx, userID, locationID, entity.OwnerTypeUserProfile)
}

// GetMerchantLocations retrieves all locations for a merchant
func (s *locationService) GetMerchantLocations(ctx context.Context, merchantID uuid.UUID) ([]*entity.Address, error) {
	return s.getLocations(ctx, merchantID, entity.OwnerTypeMerchantProfile)
}

// AddMerchantLocation adds a new location for a merchant
func (s *locationService) AddMerchantLocation(ctx context.Context, merchantID uuid.UUID, input *usecase.AddLocationInput) (*entity.Address, error) {
	return s.addLocation(ctx, merchantID, entity.OwnerTypeMerchantProfile, s.config.LocationNotification.MerchantMaxLocations, input)
}

// UpdateMerchantLocation updates an existing location for a merchant
func (s *locationService) UpdateMerchantLocation(ctx context.Context, merchantID, locationID uuid.UUID, input *usecase.UpdateLocationInput) (*entity.Address, error) {
	return s.updateLocation(ctx, merchantID, locationID, entity.OwnerTypeMerchantProfile, input)
}

// DeleteMerchantLocation deletes a location for a merchant (soft delete)
func (s *locationService) DeleteMerchantLocation(ctx context.Context, merchantID, locationID uuid.UUID) error {
	return s.deleteLocation(ctx, merchantID, locationID, entity.OwnerTypeMerchantProfile)
}

func (s *locationService) getLocations(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) ([]*entity.Address, error) {
	addresses, err := s.addressRepo.FindAddressesByOwner(ctx, ownerID, ownerType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find addresses by owner")
	}

	return addresses, nil
}

func (s *locationService) addLocation(
	ctx context.Context,
	ownerID uuid.UUID,
	ownerType entity.OwnerType,
	maxLocations int,
	input *usecase.AddLocationInput,
) (*entity.Address, error) {
	if input == nil {
		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "location input is required")
	}

	count, err := s.addressRepo.CountAddressesByOwner(ctx, ownerID, ownerType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to count addresses by owner")
	}

	if count >= int64(maxLocations) {
		return nil, ErrLocationLimitReached
	}

	address := s.newAddress(ownerID, ownerType, input)
	if err := s.addressRepo.CreateAddress(ctx, address); err != nil {
		return nil, errors.Wrap(err, "failed to create address")
	}

	return address, nil
}

func (s *locationService) updateLocation(
	ctx context.Context,
	ownerID, locationID uuid.UUID,
	ownerType entity.OwnerType,
	input *usecase.UpdateLocationInput,
) (*entity.Address, error) {
	if input == nil {
		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "location update input is required")
	}

	address, err := s.findOwnedAddress(ctx, ownerID, locationID, ownerType)
	if err != nil {
		return nil, err
	}

	s.applyAddressUpdates(address, input)
	if err := s.addressRepo.UpdateAddress(ctx, address); err != nil {
		return nil, errors.Wrap(err, "failed to update address")
	}

	return address, nil
}

func (s *locationService) deleteLocation(
	ctx context.Context,
	ownerID, locationID uuid.UUID,
	ownerType entity.OwnerType,
) error {
	if _, err := s.findOwnedAddress(ctx, ownerID, locationID, ownerType); err != nil {
		return err
	}

	if err := s.addressRepo.DeleteAddress(ctx, locationID); err != nil {
		return errors.Wrap(err, "failed to delete address")
	}

	return nil
}

func (s *locationService) findOwnedAddress(
	ctx context.Context,
	ownerID, locationID uuid.UUID,
	ownerType entity.OwnerType,
) (*entity.Address, error) {
	address, err := s.addressRepo.FindAddressByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, repository.ErrAddressNotFound) {
			return nil, ErrLocationNotFound
		}

		return nil, errors.Wrap(err, "failed to find address by ID")
	}

	if address.OwnerID != ownerID || address.OwnerType != ownerType {
		return nil, ErrUnauthorized
	}

	return address, nil
}

func (s *locationService) newAddress(ownerID uuid.UUID, ownerType entity.OwnerType, input *usecase.AddLocationInput) *entity.Address {
	return &entity.Address{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		OwnerType:   ownerType,
		Label:       input.Label,
		FullAddress: input.FullAddress,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		IsPrimary:   input.IsPrimary,
		IsActive:    input.IsActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
