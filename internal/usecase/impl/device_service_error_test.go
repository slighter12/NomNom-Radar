package impl

import (
	"context"
	"testing"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/errors"
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
		Return(nil, repository.ErrDeviceNotFound)

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.Equal(t, domainerrors.ErrDeviceNotFound, err)
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

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, errors.New("database error"))

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find device by ID")
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

	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, newToken).
		Return(errors.New("database error"))

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update FCM token")
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
		Return(nil, repository.ErrDeviceNotFound)

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.Equal(t, domainerrors.ErrDeviceNotFound, err)
}

func TestDeviceService_DeactivateDevice_FindError(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, errors.New("database error"))

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find device by ID")
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

	fx.deviceRepo.EXPECT().
		DeleteDevice(ctx, deviceID).
		Return(errors.New("database error"))

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete device")
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
	assert.Contains(t, err.Error(), "failed to find device by user and device ID")
}

func TestDeviceService_GetUserDevices_Error(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.deviceRepo.EXPECT().
		FindActiveDevicesByUser(ctx, userID).
		Return(nil, errors.New("database error"))

	devices, err := fx.service.GetUserDevices(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, devices)
	assert.Contains(t, err.Error(), "failed to find active devices by user")
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

	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, "new-fcm-token").
		Return(errors.New("database error"))

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.Contains(t, err.Error(), "failed to update FCM token")
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

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, errors.New("database error"))

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.Contains(t, err.Error(), "failed to find device by ID")
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
		Return(nil, repository.ErrDeviceNotFound)

	fx.deviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(errors.New("database error"))

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.Contains(t, err.Error(), "failed to create device")
}
