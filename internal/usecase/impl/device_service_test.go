package impl

import (
	"context"
	"errors"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDeviceService_RegisterDevice_NewDevice(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	mockDeviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{}, nil)

	mockDeviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(nil)

	device, err := service.RegisterDevice(ctx, userID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, device)
	assert.Equal(t, userID, device.UserID)
	assert.Equal(t, deviceInfo.FCMToken, device.FCMToken)
	assert.Equal(t, deviceInfo.DeviceID, device.DeviceID)
	assert.Equal(t, deviceInfo.Platform, device.Platform)
	assert.True(t, device.IsActive)
}

func TestDeviceService_RegisterDevice_UpdateExisting(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

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

	updatedDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		FCMToken: "new-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
		IsActive: true,
	}

	mockDeviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{existingDevice}, nil)

	mockDeviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, "new-fcm-token").
		Return(nil)

	mockDeviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(updatedDevice, nil)

	device, err := service.RegisterDevice(ctx, userID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, device)
	assert.Equal(t, "new-fcm-token", device.FCMToken)
}

func TestDeviceService_UpdateFCMToken_Success(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	newToken := "new-fcm-token"

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		FCMToken: "old-token",
		DeviceID: "device-123",
	}

	mockDeviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	mockDeviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, newToken).
		Return(nil)

	err := service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	require.NoError(t, err)
}

func TestDeviceService_UpdateFCMToken_NotFound(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()
	newToken := "new-fcm-token"

	mockDeviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(nil, repository.ErrDeviceNotFound)

	err := service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.Equal(t, ErrDeviceNotFound, err)
}

func TestDeviceService_UpdateFCMToken_Unauthorized(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

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

	mockDeviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	err := service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	assert.Error(t, err)
	assert.Equal(t, ErrDeviceUnauthorized, err)
}

func TestDeviceService_GetUserDevices(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	expectedDevices := []*entity.UserDevice{
		{ID: uuid.New(), UserID: userID, IsActive: true},
		{ID: uuid.New(), UserID: userID, IsActive: true},
	}

	mockDeviceRepo.EXPECT().
		FindActiveDevicesByUser(ctx, userID).
		Return(expectedDevices, nil)

	devices, err := service.GetUserDevices(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedDevices, devices)
}

func TestDeviceService_DeactivateDevice_Success(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	deviceID := uuid.New()

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		IsActive: true,
	}

	mockDeviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	mockDeviceRepo.EXPECT().
		DeleteDevice(ctx, deviceID).
		Return(nil)

	err := service.DeactivateDevice(ctx, userID, deviceID)
	require.NoError(t, err)
}

func TestDeviceService_DeactivateDevice_Unauthorized(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	differentUserID := uuid.New()
	deviceID := uuid.New()

	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   differentUserID,
		IsActive: true,
	}

	mockDeviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	err := service.DeactivateDevice(ctx, userID, deviceID)
	assert.Error(t, err)
	assert.Equal(t, ErrDeviceUnauthorized, err)
}

func TestDeviceService_RegisterDevice_FindError(t *testing.T) {
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(mockDeviceRepo)

	ctx := context.Background()
	userID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	expectedErr := errors.New("database error")
	mockDeviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return(nil, expectedErr)

	device, err := service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.Contains(t, err.Error(), "failed to find devices by user")
}
