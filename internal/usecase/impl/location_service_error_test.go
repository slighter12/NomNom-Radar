package impl

import (
	"context"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLocationService_UpdateMerchantLocation_NotFound(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, repository.ErrAddressNotFound)

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrLocationNotFound, err)
}

func TestLocationService_UpdateMerchantLocation_Unauthorized(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	differentMerchantID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   differentMerchantID,
		OwnerType: entity.OwnerTypeMerchantProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_UpdateMerchantLocation_WrongOwnerType(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   merchantID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_UpdateMerchantLocation_UpdateError(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()
	newLabel := "Updated Store"
	input := &usecase.UpdateLocationInput{
		Label: &newLabel,
	}

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   merchantID,
		OwnerType: entity.OwnerTypeMerchantProfile,
		Label:     "Store",
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	fx.addressRepo.EXPECT().
		UpdateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(errors.New("database error"))

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to update address")
}

func TestLocationService_UpdateMerchantLocation_FindError(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, errors.New("database connection failed"))

	address, err := fx.service.UpdateMerchantLocation(ctx, merchantID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to find address by ID")
}

func TestLocationService_DeleteMerchantLocation_NotFound(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, repository.ErrAddressNotFound)

	err := fx.service.DeleteMerchantLocation(ctx, merchantID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrLocationNotFound, err)
}

func TestLocationService_DeleteMerchantLocation_Unauthorized(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	differentMerchantID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   differentMerchantID,
		OwnerType: entity.OwnerTypeMerchantProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	err := fx.service.DeleteMerchantLocation(ctx, merchantID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_DeleteMerchantLocation_WrongOwnerType(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   merchantID,
		OwnerType: entity.OwnerTypeUserProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	err := fx.service.DeleteMerchantLocation(ctx, merchantID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_DeleteMerchantLocation_DeleteError(t *testing.T) {
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
		Return(errors.New("database error"))

	err := fx.service.DeleteMerchantLocation(ctx, merchantID, locationID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete address")
}

func TestLocationService_DeleteMerchantLocation_FindError(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()
	locationID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, errors.New("database connection failed"))

	err := fx.service.DeleteMerchantLocation(ctx, merchantID, locationID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find address by ID")
}

func TestLocationService_GetUserLocations_Error(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressesByOwner(ctx, userID, entity.OwnerTypeUserProfile).
		Return(nil, errors.New("database error"))

	addresses, err := fx.service.GetUserLocations(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Contains(t, err.Error(), "failed to find addresses by owner")
}

func TestLocationService_GetMerchantLocations_Error(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	merchantID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressesByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile).
		Return(nil, errors.New("database error"))

	addresses, err := fx.service.GetMerchantLocations(ctx, merchantID)
	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Contains(t, err.Error(), "failed to find addresses by owner")
}

func TestLocationService_AddUserLocation_CreateError(t *testing.T) {
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
		Return(int64(2), nil)

	fx.addressRepo.EXPECT().
		CreateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(errors.New("database error"))

	address, err := fx.service.AddUserLocation(ctx, userID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to create address")
}

func TestLocationService_AddMerchantLocation_CountError(t *testing.T) {
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
		Return(int64(0), errors.New("database error"))

	address, err := fx.service.AddMerchantLocation(ctx, merchantID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to count addresses by owner")
}

func TestLocationService_AddMerchantLocation_CreateError(t *testing.T) {
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
		Return(int64(5), nil)

	fx.addressRepo.EXPECT().
		CreateAddress(ctx, mock.AnythingOfType("*entity.Address")).
		Return(errors.New("database error"))

	address, err := fx.service.AddMerchantLocation(ctx, merchantID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to create address")
}

func TestLocationService_UpdateUserLocation_UpdateError(t *testing.T) {
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
		Return(errors.New("database error"))

	address, err := fx.service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to update address")
}

func TestLocationService_UpdateUserLocation_FindError(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, errors.New("database connection failed"))

	address, err := fx.service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to find address by ID")
}

func TestLocationService_UpdateUserLocation_WrongOwnerType(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()
	input := &usecase.UpdateLocationInput{}

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   userID,
		OwnerType: entity.OwnerTypeMerchantProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	address, err := fx.service.UpdateUserLocation(ctx, userID, locationID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Equal(t, ErrUnauthorized, err)
}

func TestLocationService_DeleteUserLocation_NotFound(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, repository.ErrAddressNotFound)

	err := fx.service.DeleteUserLocation(ctx, userID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrLocationNotFound, err)
}

func TestLocationService_DeleteUserLocation_DeleteError(t *testing.T) {
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
		Return(errors.New("database error"))

	err := fx.service.DeleteUserLocation(ctx, userID, locationID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete address")
}

func TestLocationService_DeleteUserLocation_FindError(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(nil, errors.New("database connection failed"))

	err := fx.service.DeleteUserLocation(ctx, userID, locationID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find address by ID")
}

func TestLocationService_DeleteUserLocation_WrongOwnerType(t *testing.T) {
	fx := createTestLocationService(t, nil)

	ctx := context.Background()
	userID := uuid.New()
	locationID := uuid.New()

	existingAddress := &entity.Address{
		ID:        locationID,
		OwnerID:   userID,
		OwnerType: entity.OwnerTypeMerchantProfile,
	}

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, locationID).
		Return(existingAddress, nil)

	err := fx.service.DeleteUserLocation(ctx, userID, locationID)
	assert.Error(t, err)
	assert.Equal(t, ErrUnauthorized, err)
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

func TestLocationService_AddUserLocation_CountError(t *testing.T) {
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
		Return(int64(0), errors.New("database error"))

	address, err := fx.service.AddUserLocation(ctx, userID, input)
	assert.Error(t, err)
	assert.Nil(t, address)
	assert.Contains(t, err.Error(), "failed to count addresses by owner")
}
