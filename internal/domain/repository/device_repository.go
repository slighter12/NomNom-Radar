// Package repository defines the interfaces for the persistence layer.
package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Domain-specific errors for device persistence.
var (
	// ErrDeviceNotFound is returned when a device is not found.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrDuplicateDevice is returned when trying to create a device that already exists.
	ErrDuplicateDevice = errors.New("device already exists")
)

// DeviceRepository defines the interface for device-related database operations.
type DeviceRepository interface {
	// CreateDevice persists a new device for a user.
	CreateDevice(ctx context.Context, device *entity.UserDevice) error

	// FindDeviceByID retrieves a device by its unique ID.
	FindDeviceByID(ctx context.Context, id uuid.UUID) (*entity.UserDevice, error)

	// FindDevicesByUser retrieves all devices for a specific user (including inactive).
	FindDevicesByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error)

	// FindActiveDevicesByUser retrieves all active devices for a specific user.
	FindActiveDevicesByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error)

	// UpdateFCMToken updates the FCM token for a specific device.
	UpdateFCMToken(ctx context.Context, deviceID uuid.UUID, fcmToken string) error

	// DeleteDevice removes a device by its ID (soft delete).
	DeleteDevice(ctx context.Context, id uuid.UUID) error
}
