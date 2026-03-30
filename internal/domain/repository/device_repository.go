package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// DeviceRepository defines the interface for device-related database operations.
type DeviceRepository interface {
	// CreateDevice persists a new device for a user.
	CreateDevice(ctx context.Context, device *entity.UserDevice) error

	// FindDeviceByID retrieves a device by its unique ID.
	FindDeviceByID(ctx context.Context, id uuid.UUID) (*entity.UserDevice, error)

	// FindDeviceByUserAndDeviceID retrieves a device by user ID and client device ID.
	FindDeviceByUserAndDeviceID(ctx context.Context, userID uuid.UUID, deviceID string) (*entity.UserDevice, error)

	// FindDevicesByUser retrieves all devices for a specific user (including inactive).
	FindDevicesByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error)

	// FindActiveDevicesByUser retrieves all active devices for a specific user.
	FindActiveDevicesByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error)

	// UpdateFCMToken updates the FCM token for a specific device.
	UpdateFCMToken(ctx context.Context, deviceID uuid.UUID, fcmToken string) error

	// DeleteDevice removes a device by its ID (soft delete).
	DeleteDevice(ctx context.Context, id uuid.UUID) error
}
