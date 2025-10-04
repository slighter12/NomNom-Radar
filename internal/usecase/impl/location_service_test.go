package impl

import (
	"context"
	"errors"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLocationService_GetUserLocations(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	userID := uuid.New()
	expectedAddresses := []*entity.Address{
		{ID: uuid.New(), OwnerID: userID, OwnerType: entity.OwnerTypeUserProfile},
	}

	mockAddressRepo.EXPECT().
		FindAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(expectedAddresses, nil)

	addresses, err := service.GetUserLocations(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedAddresses, addresses)
}

func TestLocationService_AddUserLocation_Success(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			UserMaxLocations: 5,
		},
	}
	service := NewLocationService(mockAddressRepo, cfg)

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

	mockAddressRepo.EXPECT().
		CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(int64(2), nil)

	mockAddressRepo.EXPECT().
		CreateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := service.AddUserLocation(ctx, userID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, userID, address.OwnerID)
	assert.Equal(t, entity.OwnerTypeUserProfile, address.OwnerType)
	assert.Equal(t, input.Label, address.Label)
}

func TestLocationService_AddUserLocation_LimitReached(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			UserMaxLocations: 5,
		},
	}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Home",
		FullAddress: "123 Main St",
		Latitude:    25.0,
		Longitude:   121.0,
	}

	mockAddressRepo.EXPECT().
		CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(int64(5), nil)

	address, err := service.AddUserLocation(ctx, userID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationLimitReached, err)
}

func TestLocationService_UpdateUserLocation_Success(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

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

	mockAddressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	mockAddressRepo.EXPECT().
		UpdateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := service.UpdateUserLocation(ctx, userID, locationID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, newLabel, address.Label)
}

func TestLocationService_UpdateUserLocation_NotFound(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	mockAddressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, repository.ErrAddressNotFound)

	address, err := service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationNotFound, err)
}

func TestLocationService_UpdateUserLocation_Unauthorized(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

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

	mockAddressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	address, err := service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_DeleteUserLocation_Success(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   userID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	mockAddressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	mockAddressRepo.EXPECT().
		DeleteAddress(ctx, locationID).
		Return(nil)

	err := service.DeleteUserLocation(ctx, userID, locationID)
	require.NoError(t, err)
}

func TestLocationService_DeleteUserLocation_Unauthorized(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	userID := uuid.New()
	differentUserID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   differentUserID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	mockAddressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	err := service.DeleteUserLocation(ctx, userID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_GetMerchantLocations(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	merchantID := uuid.New()
	expectedAddresses := []*entity.Address{
		{ID: uuid.New(), OwnerID: merchantID, OwnerType: entity.OwnerTypeMerchantProfile},
	}

	mockAddressRepo.EXPECT().
		FindAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(expectedAddresses, nil)

	addresses, err := service.GetMerchantLocations(ctx, merchantID)
	require.NoError(t, err)
	assert.Equal(t, expectedAddresses, addresses)
}

func TestLocationService_AddMerchantLocation_Success(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			MerchantMaxLocations: 10,
		},
	}
	service := NewLocationService(mockAddressRepo, cfg)

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

	mockAddressRepo.EXPECT().
		CountAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(int64(5), nil)

	mockAddressRepo.EXPECT().
		CreateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(nil)

	address, err := service.AddMerchantLocation(ctx, merchantID, input)
	require.NoError(t, err)
	assert.NotNil(t, address)
	assert.Equal(t, merchantID, address.OwnerID)
	assert.Equal(t, entity.OwnerTypeMerchantProfile, address.OwnerType)
}

func TestLocationService_AddMerchantLocation_LimitReached(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			MerchantMaxLocations: 10,
		},
	}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	merchantID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Store",
		FullAddress: "456 Business Ave",
		Latitude:    25.0,
		Longitude:   121.0,
	}

	mockAddressRepo.EXPECT().
		CountAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(int64(10), nil)

	address, err := service.AddMerchantLocation(ctx, merchantID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationLimitReached, err)
}

func TestLocationService_CountError(t *testing.T) {
	mockAddressRepo := mockRepo.NewMockAddressRepository(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			UserMaxLocations: 5,
		},
	}
	service := NewLocationService(mockAddressRepo, cfg)

	ctx := context.Background()
	userID := uuid.New()
	input := &usecase.AddLocationInput{
		Label:       "Home",
		FullAddress: "123 Main St",
		Latitude:    25.0,
		Longitude:   121.0,
	}

	expectedErr := errors.New("database error")
	mockAddressRepo.EXPECT().
		CountAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(int64(0), expectedErr)

	address, err := service.AddUserLocation(ctx, userID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to count addresses by owner")
}
