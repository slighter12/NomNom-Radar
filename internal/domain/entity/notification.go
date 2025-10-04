// Package entity contains the core business objects of the project.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// MerchantLocationNotification represents a location notification published by a merchant.
type MerchantLocationNotification struct {
	ID           uuid.UUID  `json:"id"`            // The Global Unique Identifier (GUID) for the notification.
	MerchantID   uuid.UUID  `json:"merchant_id"`   // The ID of the merchant who published this notification.
	AddressID    *uuid.UUID `json:"address_id"`    // Optional reference to a saved address (if using a saved location).
	LocationName string     `json:"location_name"` // The name/label of the location.
	FullAddress  string     `json:"full_address"`  // The full address of the location.
	Latitude     float64    `json:"latitude"`      // The geographic latitude of the location.
	Longitude    float64    `json:"longitude"`     // The geographic longitude of the location.
	HintMessage  string     `json:"hint_message"`  // Optional hint message (e.g., "I'm at the first parking spot by the corner").
	TotalSent    int        `json:"total_sent"`    // Total number of notifications successfully sent.
	TotalFailed  int        `json:"total_failed"`  // Total number of notifications that failed to send.
	PublishedAt  time.Time  `json:"published_at"`  // Timestamp of when the notification was published.
	CreatedAt    time.Time  `json:"created_at"`    // Timestamp of when this record was created.
	UpdatedAt    time.Time  `json:"updated_at"`    // Timestamp of the last modification.
}

// NotificationLog represents a log entry for a single notification sent to a user device.
type NotificationLog struct {
	ID             uuid.UUID `json:"id"`              // The Global Unique Identifier (GUID) for the log entry.
	NotificationID uuid.UUID `json:"notification_id"` // The ID of the notification this log belongs to.
	UserID         uuid.UUID `json:"user_id"`         // The ID of the user who received the notification.
	DeviceID       uuid.UUID `json:"device_id"`       // The ID of the device that received the notification.
	Status         string    `json:"status"`          // The status of the notification (sent, failed).
	FCMMessageID   string    `json:"fcm_message_id"`  // The Firebase Cloud Messaging message ID.
	ErrorMessage   string    `json:"error_message"`   // Error message if the notification failed.
	SentAt         time.Time `json:"sent_at"`         // Timestamp of when the notification was sent.
}
