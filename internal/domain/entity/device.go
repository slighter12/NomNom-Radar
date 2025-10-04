// Package entity contains the core business objects of the project.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserDevice represents a user's device registered for push notifications.
type UserDevice struct {
	ID        uuid.UUID `json:"id"`         // The Global Unique Identifier (GUID) for the device.
	UserID    uuid.UUID `json:"user_id"`    // The ID of the user who owns this device.
	FCMToken  string    `json:"fcm_token"`  // Firebase Cloud Messaging token for push notifications.
	DeviceID  string    `json:"device_id"`  // Unique device identifier from the client.
	Platform  string    `json:"platform"`   // Device platform (ios, android).
	IsActive  bool      `json:"is_active"`  // Indicates if this device is active for notifications.
	CreatedAt time.Time `json:"created_at"` // Timestamp of when this device was registered.
	UpdatedAt time.Time `json:"updated_at"` // Timestamp of the last modification.
}
