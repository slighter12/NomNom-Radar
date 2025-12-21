package impl

import (
	"context"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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
	service := NewLocationService(addressRepo, cfg)

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

func TestLocationService_AddUserLocation_LimitReached(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Home",
		FullAddress: "123 Main St",
		Latitude:    25.0,
		Longitude:   121.0,
	}

	fx.addressRepo.EXPECT().
		CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(int64(5), nil)

	address, err := fx.service.AddUserLocation(ctx, userID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationLimitReached, err)
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

func TestLocationService_UpdateUserLocation_NotFound(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, repository.ErrAddressNotFound)

	address, err := fx.service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationNotFound, err)
}

func TestLocationService_UpdateUserLocation_Unauthorized(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	differentUserID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   differentUserID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	address, err := fx.service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrUnauthorized, err)
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

func TestLocationService_DeleteUserLocation_Unauthorized(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	differentUserID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   differentUserID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	err := fx.service.DeleteUserLocation(ctx, userID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrUnauthorized, err)
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

func TestLocationService_AddMerchantLocation_LimitReached(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Store",
		FullAddress: "456 Business Ave",
		Latitude:    25.0,
		Longitude:   121.0,
	}

	fx.addressRepo.EXPECT().
		CountAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(int64(10), nil)

	address, err := fx.service.AddMerchantLocation(ctx, merchantID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationLimitReached, err)
}

func TestLocationService_CountError(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Home",
		FullAddress: "123 Main St",
		Latitude:    25.0,
		Longitude:   121.0,
	}

	expectedErr := errors.New("database error")
	fx.addressRepo.EXPECT().
		CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(int64(0), expectedErr)

	address, err := fx.service.AddUserLocation(ctx, userID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to count addresses by owner")
}
