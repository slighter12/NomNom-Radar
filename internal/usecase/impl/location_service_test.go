package impl

import (
	"context"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// locationServiceFixtures holds all test dependencies for location service tests.
type locationServiceFixtures struct {
	service     usecase.LocationUsecase
	addressRepo *mockRepo.MockAddressRepository
}

func createTestLocationService(t *testing.T, cfg *config.Config) locationServiceFixtures {
	addressRepo := mockRepo.NewMockAddressRepository(t)
	if cfg == nil {
		cfg = &config.Config{
			LocationNotification: &config.LocationNotificationConfig{
				UserMaxLocations:     5,
				MerchantMaxLocations: 10,
			},
		}
	}
	service := NewLocationService(LocationServiceParams{
		AddressRepo: addressRepo,
		Config:      cfg,
	})

	return locationServiceFixtures{
		service:     service,
		addressRepo: addressRepo,
	}
}

func TestLocationService_GetUserLocations(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	expectedAddresses := []*entity.Address{
		{ID: uuid.New(), OwnerID: userID, OwnerType: entity.OwnerTypeUserProfile},
	}

	fx.addressRepo.EXPECT().
		FindAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(expectedAddresses, nil)

	addresses, err := fx.service.GetUserLocations(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedAddresses, addresses)
}

func TestLocationService_AddUserLocation_Success(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Home",
		FullAddress: "123 Main St",
		Latitude:    25.0,
		Longitude:   121.0,
		IsPrimary:   true,
		IsActive:    true,
	}

	fx.addressRepo.EXPECT().
		CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(int64(2), nil)

	fx.addressRepo.EXPECT().
		CreateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := fx.service.AddUserLocation(ctx, userID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, userID, address.OwnerID)
	assert.Equal(t, entity.OwnerTypeUserProfile, address.OwnerType)
	assert.Equal(t, input.Label, address.Label)
}

func TestLocationService_UpdateUserLocation_Success(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()
	newLabel := "Updated Home"
	input := &usecase.UpdateLocationInput{
		Label: &newLabel,
	}

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   userID,
		OwnerType: entity.OwnerTypeUserProfile,
		Label:     "Home",
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	fx.addressRepo.EXPECT().
		UpdateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := fx.service.UpdateUserLocation(ctx, userID, locationID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, newLabel, address.Label)
}

func TestLocationService_DeleteUserLocation_Success(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   userID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	fx.addressRepo.EXPECT().
		DeleteAddress(ctx, locationID).
		Return(nil)

	err := fx.service.DeleteUserLocation(ctx, userID, locationID)
	require.NoError(t, err)
}

func TestLocationService_GetMerchantLocations(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	expectedAddresses := []*entity.Address{
		{ID: uuid.New(), OwnerID: merchantID, OwnerType: entity.OwnerTypeMerchantProfile},
	}

	fx.addressRepo.EXPECT().
		FindAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(expectedAddresses, nil)

	addresses, err := fx.service.GetMerchantLocations(ctx, merchantID)
	require.NoError(t, err)
	assert.Equal(t, expectedAddresses, addresses)
}

func TestLocationService_AddMerchantLocation_Success(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Store",
		FullAddress: "456 Business Ave",
		Latitude:    25.0,
		Longitude:   121.0,
		IsPrimary:   true,
		IsActive:    true,
	}

	fx.addressRepo.EXPECT().
		CountAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(int64(5), nil)

	fx.addressRepo.EXPECT().
		CreateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := fx.service.AddMerchantLocation(ctx, merchantID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, merchantID, address.OwnerID)
	assert.Equal(t, entity.OwnerTypeMerchantProfile, address.OwnerType)
}

func TestLocationService_UpdateMerchantLocation_Success(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()
	newLabel := "Updated Store"
	newAddress := "789 New Ave"
	input := &usecase.UpdateLocationInput{
		Label:       &newLabel,
		FullAddress: &newAddress,
	}

	existingAddress := &entity.Address{
		ID:          locationID,
		OwnerID:     merchantID,
		OwnerType:   entity.OwnerTypeMerchantProfile,
		Label:       "Store",
		FullAddress: "456 Old St",
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	fx.addressRepo.EXPECT().
		UpdateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, newLabel, address.Label)
	assert.Equal(t, newAddress, address.FullAddress)
}

func TestLocationService_DeleteMerchantLocation_Success(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   merchantID,
		OwnerType: entity.OwnerTypeMerchantProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	fx.addressRepo.EXPECT().
		DeleteAddress(ctx, locationID).
		Return(nil)

	err := fx.service.DeleteMerchantLocation(ctx, merchantID, locationID)
	require.NoError(t, err)
}

func TestLocationService_UpdateMerchantLocation_AllFields(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()

	newLabel := "Updated Store"
	newAddress := "789 New Ave"
	newLat := 26.0
	newLng := 122.0
	newIsPrimary := true
	newIsActive := false

	input := &usecase.UpdateLocationInput{
		Label:       &newLabel,
		FullAddress: &newAddress,
		Latitude:    &newLat,
		Longitude:   &newLng,
		IsPrimary:   &newIsPrimary,
		IsActive:    &newIsActive,
	}

	existingAddress := &entity.Address{
		ID:          locationID,
		OwnerID:     merchantID,
		OwnerType:   entity.OwnerTypeMerchantProfile,
		Label:       "Store",
		FullAddress: "456 Old St",
		Latitude:    25.0,
		Longitude:   121.0,
		IsPrimary:   false,
		IsActive:    true,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	fx.addressRepo.EXPECT().
		UpdateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, newLabel, address.Label)
	assert.Equal(t, newAddress, address.FullAddress)
	assert.Equal(t, newLat, address.Latitude)
	assert.Equal(t, newLng, address.Longitude)
	assert.Equal(t, newIsPrimary, address.IsPrimary)
	assert.Equal(t, newIsActive, address.IsActive)
}

func TestLocationService_NewLocationService_NilConfig(t *testing.T) {
	addressRepo := mockRepo.NewMockAddressRepository(t)

	// Test with nil LocationNotification config - should use defaults
	cfg := &config.Config{
		LocationNotification: nil,
	}

	service := NewLocationService(LocationServiceParams{
		AddressRepo: addressRepo,
		Config:      cfg,
	})

	assert.NotNil(t, service)
}
