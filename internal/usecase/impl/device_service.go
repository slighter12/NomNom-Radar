package impl

import (
	"context"
	"time"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

var (
	// ErrDeviceNotFound is returned when a device is not found
	ErrDeviceNotFound = errors.New("device not found")
	// ErrDeviceUnauthorized is returned when a user tries to access a device they don't own
	ErrDeviceUnauthorized = errors.New("unauthorized to access this device")
)

type deviceService struct {
	deviceRepo repository.DeviceRepository
}

// DeviceServiceParams holds dependencies for DeviceService, injected by Fx.
type DeviceServiceParams struct {
	fx.In

	DeviceRepo repository.DeviceRepository
}

// NewDeviceService creates a new device service instance
func NewDeviceService(params DeviceServiceParams) usecase.DeviceUsecase {
	return &deviceService{
		deviceRepo: params.DeviceRepo,
	}
}

// RegisterDevice registers a new device or updates an existing one
func (s *deviceService) RegisterDevice(ctx context.Context, userID uuid.UUID, deviceInfo *usecase.DeviceInfo) (*entity.UserDevice, error) {
	// Check if device already exists for this user
	devices, err := s.deviceRepo.FindDevicesByUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find devices by user")
	}

	// Look for existing device with same device_id
	for _, device := range devices {
		if device.DeviceID == deviceInfo.DeviceID {
			// Update FCM token for existing device
			if err := s.deviceRepo.UpdateFCMToken(ctx, device.ID, deviceInfo.FCMToken); err != nil {
				return nil, errors.Wrap(err, "failed to update FCM token")
			}
			// Fetch and return updated device
			updatedDevice, err := s.deviceRepo.FindDeviceByID(ctx, device.ID)
			if err != nil {
				return nil, errors.Wrap(err, "failed to find device by ID")
			}

			return updatedDevice, nil
		}
	}

	// Create new device
	device := &entity.UserDevice{
		ID:        uuid.New(),
		UserID:    userID,
		FCMToken:  deviceInfo.FCMToken,
		DeviceID:  deviceInfo.DeviceID,
		Platform:  deviceInfo.Platform,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.deviceRepo.CreateDevice(ctx, device); err != nil {
		return nil, errors.Wrap(err, "failed to create device")
	}

	return device, nil
}

// UpdateFCMToken updates the FCM token for a specific device
func (s *deviceService) UpdateFCMToken(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, fcmToken string) error {
	// Fetch device to verify ownership
	device, err := s.deviceRepo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, repository.ErrDeviceNotFound) {
			return ErrDeviceNotFound
		}

		return errors.Wrap(err, "failed to find device by ID")
	}

	// Verify ownership
	if device.UserID != userID {
		return ErrDeviceUnauthorized
	}

	if err := s.deviceRepo.UpdateFCMToken(ctx, deviceID, fcmToken); err != nil {
		return errors.Wrap(err, "failed to update FCM token")
	}

	return nil
}

// GetUserDevices retrieves all active devices for a user
func (s *deviceService) GetUserDevices(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error) {
	devices, err := s.deviceRepo.FindActiveDevicesByUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find active devices by user")
	}

	return devices, nil
}

// DeactivateDevice deactivates a device (soft delete)
func (s *deviceService) DeactivateDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	// Fetch device to verify ownership
	device, err := s.deviceRepo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, repository.ErrDeviceNotFound) {
			return ErrDeviceNotFound
		}

		return errors.Wrap(err, "failed to find device by ID")
	}

	// Verify ownership
	if device.UserID != userID {
		return ErrDeviceUnauthorized
	}

	if err := s.deviceRepo.DeleteDevice(ctx, deviceID); err != nil {
		return errors.Wrap(err, "failed to delete device")
	}

	return nil
}
