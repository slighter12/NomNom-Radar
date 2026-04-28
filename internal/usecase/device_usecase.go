package usecase

import (
	"context"
	"time"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// DeviceInfo represents device information for registration
type DeviceInfo struct {
	FCMToken string `json:"fcm_token"`
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

// DeviceHealthStatus is the client-facing health state of a user device.
type DeviceHealthStatus string

const (
	DeviceHealthStatusHealthy DeviceHealthStatus = "healthy"
	DeviceHealthStatusStale   DeviceHealthStatus = "stale"
	DeviceHealthStatusInvalid DeviceHealthStatus = "invalid"
)

// DeviceHealthInfo represents the computed health state of a user device.
type DeviceHealthInfo struct {
	ID               uuid.UUID          `json:"id"`
	ClientDeviceID   string             `json:"client_device_id"`
	HealthStatus     DeviceHealthStatus `json:"health_status"`
	TokenRefreshedAt time.Time          `json:"token_refreshed_at"`
	RequiresRebind   bool               `json:"requires_rebind"`
}

// DeviceUsecase defines the interface for device management use cases
type DeviceUsecase interface {
	// RegisterDevice registers a new device or updates an existing one
	RegisterDevice(ctx context.Context, userID uuid.UUID, deviceInfo *DeviceInfo) (*entity.UserDevice, error)

	// UpdateFCMToken updates the FCM token for a specific device
	UpdateFCMToken(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, fcmToken string) error

	// GetUserDevices retrieves active devices with healthy push tokens for a user.
	GetUserDevices(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error)

	// GetDeviceHealth retrieves computed device health information for a user.
	GetDeviceHealth(ctx context.Context, userID uuid.UUID) ([]*DeviceHealthInfo, error)

	// DeactivateDevice deactivates a device without soft-deleting it.
	DeactivateDevice(ctx context.Context, userID, deviceID uuid.UUID) error
}
