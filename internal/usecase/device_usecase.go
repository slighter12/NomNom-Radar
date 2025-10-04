package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// DeviceInfo represents device information for registration
type DeviceInfo struct {
	FCMToken string `json:"fcm_token"`
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

// DeviceUsecase defines the interface for device management use cases
type DeviceUsecase interface {
	// RegisterDevice registers a new device or updates an existing one
	RegisterDevice(ctx context.Context, userID uuid.UUID, deviceInfo *DeviceInfo) (*entity.UserDevice, error)

	// UpdateFCMToken updates the FCM token for a specific device
	UpdateFCMToken(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, fcmToken string) error

	// GetUserDevices retrieves all active devices for a user
	GetUserDevices(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error)

	// DeactivateDevice deactivates a device (soft delete)
	DeactivateDevice(ctx context.Context, userID, deviceID uuid.UUID) error
}
