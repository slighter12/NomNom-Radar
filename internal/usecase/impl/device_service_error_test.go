package impl

import (
	"context"
	"errors"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeviceService_UpdateFCMToken_NotFound(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	newToken := "new-fcm-token"

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, domainerrors.ErrDeviceNotFound)

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrDeviceNotFound)
}

func TestDeviceService_UpdateFCMToken_Unauthorized(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	differentUserID := uuid.New()
	deviceID := uuid.New()
	newToken := "new-fcm-token"

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   differentUserID,
		FCMToken: "old-token",
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.Equal(t, domainerrors.ErrDeviceOwnershipViolation, err)
}

func TestDeviceService_UpdateFCMToken_FindError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	newToken := "new-fcm-token"

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, expectedErr)

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_UpdateFCMToken_UpdateError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	newToken := "new-fcm-token"

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		FCMToken: "old-token",
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, newToken).
		Return(expectedErr)

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_DeactivateDevice_Unauthorized(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	differentUserID := uuid.New()
	deviceID := uuid.New()

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   differentUserID,
		IsActive: true,
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.Equal(t, domainerrors.ErrDeviceOwnershipViolation, err)
}

func TestDeviceService_DeactivateDevice_NotFound(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, domainerrors.ErrDeviceNotFound)

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrDeviceNotFound)
}

func TestDeviceService_DeactivateDevice_FindError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, expectedErr)

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_DeactivateDevice_DeleteError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		IsActive: true,
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		DeleteDevice(ctx, deviceID).
		Return(expectedErr)

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_RegisterDevice_FindError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		FindDeviceByUserAndDeviceID(ctx, userID, "device-123").
		Return(nil, expectedErr)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_GetUserDevices_Error(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		FindActiveDevicesByUser(ctx, userID).
		Return(nil, expectedErr)

	devices, err := fx.service.GetUserDevices(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, devices)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_RegisterDevice_UpdateExisting_UpdateError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		FCMToken: "old-token",
		DeviceID: "device-123",
		Platform: "ios",
		IsActive: true,
	}

	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "new-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByUserAndDeviceID(ctx, userID, "device-123").
		Return(existingDevice, nil)

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, "new-fcm-token").
		Return(expectedErr)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_RegisterDevice_UpdateExisting_FindByIDError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		FCMToken: "old-token",
		DeviceID: "device-123",
		Platform: "ios",
		IsActive: true,
	}

	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "new-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByUserAndDeviceID(ctx, userID, "device-123").
		Return(existingDevice, nil)

	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, "new-fcm-token").
		Return(nil)

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, expectedErr)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeviceService_RegisterDevice_NewDevice_CreateError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.deviceRepo.EXPECT().
		FindDeviceByUserAndDeviceID(ctx, userID, "device-123").
		Return(nil, domainerrors.ErrDeviceNotFound)

	expectedErr := errors.New("database error")
	fx.deviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(expectedErr)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.ErrorIs(t, err, expectedErr)
}
