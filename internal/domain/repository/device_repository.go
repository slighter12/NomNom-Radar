package repository

import (
	"context"
	"time"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// DeviceListFilter defines filter options when listing user devices.
type DeviceListFilter struct {
	IncludeDeleted    bool
	OnlyDeleted       bool
	OnlyHealthy       bool
	HealthyWindowDays int
}

// DeviceHealthRecord defines the projection fields used to compute device health.
type DeviceHealthRecord struct {
	ID               uuid.UUID
	ClientDeviceID   string
	TokenRefreshedAt time.Time
	IsDeleted        bool
}

// DeviceRepository defines the interface for device-related database operations.
type DeviceRepository interface {
	// CreateDevice persists a new device for a user.
	CreateDevice(ctx context.Context, device *entity.UserDevice) error

	// FindDeviceByID retrieves a device by its unique ID.
	FindDeviceByID(ctx context.Context, id uuid.UUID) (*entity.UserDevice, error)

	// FindDeviceByUserAndDeviceID retrieves a device by user ID and client device ID.
	FindDeviceByUserAndDeviceID(ctx context.Context, userID uuid.UUID, deviceID string) (*entity.UserDevice, error)

	// FindDevicesByUser retrieves devices for a specific user based on the provided filter.
	FindDevicesByUser(ctx context.Context, userID uuid.UUID, filter DeviceListFilter) ([]*entity.UserDevice, error)

	// FindDeviceHealthByUser retrieves health projection fields for a user's devices, including soft-deleted records.
	FindDeviceHealthByUser(ctx context.Context, userID uuid.UUID) ([]DeviceHealthRecord, error)

	// FindDeviceByUserAndDeviceIDIncludingDeleted retrieves a device by user ID and client device ID, including soft-deleted records.
	FindDeviceByUserAndDeviceIDIncludingDeleted(ctx context.Context, userID uuid.UUID, deviceID string) (*entity.UserDevice, error)

	// UpdateFCMToken updates the FCM token for a specific device.
	UpdateFCMToken(ctx context.Context, deviceID uuid.UUID, fcmToken string) error

	// SetDeviceActive updates the device active state without soft-deleting it.
	SetDeviceActive(ctx context.Context, id uuid.UUID, isActive bool) error

	// RestoreAndUpdateDevice restores a soft-deleted device owned by a user and refreshes its token state.
	RestoreAndUpdateDevice(ctx context.Context, userID, id uuid.UUID, fcmToken string) error

	// SoftDeleteStaleDevices soft-deletes devices whose token freshness exceeds the provided threshold.
	SoftDeleteStaleDevices(ctx context.Context, staleDays int) (int64, error)

	// DeleteDevice removes a device by its ID (soft delete).
	DeleteDevice(ctx context.Context, id uuid.UUID) error
}
