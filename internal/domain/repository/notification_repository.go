// Package repository defines the interfaces for the persistence layer.
package repository

import (
	"context"
	"errors"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// Domain-specific errors for notification persistence.
var (
	// ErrNotificationNotFound is returned when a notification is not found.
	ErrNotificationNotFound = errors.New("notification not found")
	// ErrNotificationLogNotFound is returned when a notification log is not found.
	ErrNotificationLogNotFound = errors.New("notification log not found")
)

// NotificationRepository defines the interface for notification-related database operations.
type NotificationRepository interface {
	// CreateNotification persists a new merchant location notification.
	CreateNotification(ctx context.Context, notification *entity.MerchantLocationNotification) error

	// FindNotificationByID retrieves a notification by its unique ID.
	FindNotificationByID(ctx context.Context, id uuid.UUID) (*entity.MerchantLocationNotification, error)

	// FindNotificationsByMerchant retrieves all notifications for a specific merchant.
	FindNotificationsByMerchant(ctx context.Context, merchantID uuid.UUID, limit, offset int) ([]*entity.MerchantLocationNotification, error)

	// UpdateNotificationStatus updates the total sent and failed counts for a notification.
	UpdateNotificationStatus(ctx context.Context, id uuid.UUID, totalSent, totalFailed int) error

	// CreateNotificationLog persists a single notification log entry.
	CreateNotificationLog(ctx context.Context, log *entity.NotificationLog) error

	// BatchCreateNotificationLogs persists multiple notification log entries in a batch for better performance.
	BatchCreateNotificationLogs(ctx context.Context, logs []*entity.NotificationLog) error
}
