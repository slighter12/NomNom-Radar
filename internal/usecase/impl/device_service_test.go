package impl

import (
	"context"
	"testing"

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

// deviceServiceFixtures holds all test dependencies for device service tests.
type deviceServiceFixtures struct {
	service    usecase.DeviceUsecase
	deviceRepo *mockRepo.MockDeviceRepository
}

func createTestDeviceService(t *testing.T) deviceServiceFixtures {
	deviceRepo := mockRepo.NewMockDeviceRepository(t)
	service := NewDeviceService(deviceRepo)

	return deviceServiceFixtures{
		service:    service,
		deviceRepo: deviceRepo,
	}
}

func TestDeviceService_RegisterDevice_NewDevice(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.deviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{}, nil)

	fx.deviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(nil)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, device)
	assert.Equal(t, userID, device.UserID)
	assert.Equal(t, deviceInfo.FCMToken, device.FCMToken)
	assert.Equal(t, deviceInfo.DeviceID, device.DeviceID)
	assert.Equal(t, deviceInfo.Platform, device.Platform)
	assert.True(t, device.IsActive)
}

func TestDeviceService_RegisterDevice_UpdateExisting(t *testing.T) {
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

	updatedDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		FCMToken: "new-fcm-token",
		DeviceID: "device-123",
		Platform: "ios",
		IsActive: true,
	}

	fx.deviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{existingDevice}, nil)

	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, "new-fcm-token").
		Return(nil)

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(updatedDevice, nil)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, device)
	assert.Equal(t, "new-fcm-token", device.FCMToken)
}

func TestDeviceService_UpdateFCMToken_Success(t *testing.T) {
	fx := createTestDeviceService(t)

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

	fx.deviceRepo.EXPECT().
		FindDeviceByID(ctx, deviceID).
		Return(existingDevice, nil)

	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, newToken).
		Return(nil)

	err := fx.service.UpdateFCMToken(ctx, userID, deviceID, newToken)
	require.NoError(t, err)
}

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
	assert.Equal(t, ErrDeviceNotFound, err)
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
	assert.Equal(t, ErrDeviceUnauthorized, err)
}

func TestDeviceService_GetUserDevices(t *testing.T) {
	fx := createTestDeviceService(t)

	ctx := context.Background()
	userID := uuid.New()
	expectedDevices := []*entity.UserDevice{
		{ID: uuid.New(), UserID: userID, IsActive: true},
		{ID: uuid.New(), UserID: userID, IsActive: true},
	}

	fx.deviceRepo.EXPECT().
		FindActiveDevicesByUser(ctx, userID).
		Return(expectedDevices, nil)

	devices, err := fx.service.GetUserDevices(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedDevices, devices)
}

func TestDeviceService_DeactivateDevice_Success(t *testing.T) {
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
		Return(nil)

	err := fx.service.DeactivateDevice(ctx, userID, deviceID)
	require.NoError(t, err)
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
	assert.Equal(t, ErrDeviceUnauthorized, err)
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
		FindDevicesByUser(ctx, userID).
		Return(nil, expectedErr)

	device, err := fx.service.RegisterDevice(ctx, userID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, device)
	assert.Contains(t, err.Error(), "failed to find devices by user")
}
