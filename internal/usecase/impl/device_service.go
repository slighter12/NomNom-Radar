package impl

import (
	"context"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
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
	return upsertUserDevice(ctx, s.deviceRepo, userID, deviceInfo)
}

// UpdateFCMToken updates the FCM token for a specific device
func (s *deviceService) UpdateFCMToken(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, fcmToken string) error {
	if err := s.ensureOwnedDevice(ctx, userID, deviceID); err != nil {
		return err
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
	if err := s.ensureOwnedDevice(ctx, userID, deviceID); err != nil {
		return err
	}

	if err := s.deviceRepo.DeleteDevice(ctx, deviceID); err != nil {
		return errors.Wrap(err, "failed to delete device")
	}

	return nil
}

func (s *deviceService) ensureOwnedDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	device, err := s.deviceRepo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, repository.ErrDeviceNotFound) {
			return domainerrors.ErrDeviceNotFound
		}

		return errors.Wrap(err, "failed to find device by ID")
	}

	if device.UserID != userID {
		return domainerrors.ErrDeviceOwnershipViolation
	}

	return nil
}
