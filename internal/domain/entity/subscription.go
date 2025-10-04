// Package entity contains the core business objects of the project.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserMerchantSubscription represents a user's subscription to a merchant for location notifications.
type UserMerchantSubscription struct {
	ID                 uuid.UUID `json:"id"`                  // The Global Unique Identifier (GUID) for the subscription.
	UserID             uuid.UUID `json:"user_id"`             // The ID of the user who subscribed.
	MerchantID         uuid.UUID `json:"merchant_id"`         // The ID of the merchant being subscribed to.
	IsActive           bool      `json:"is_active"`           // Indicates if this subscription is active.
	NotificationRadius float64   `json:"notification_radius"` // The radius (in meters) within which the user wants to receive notifications.
	SubscribedAt       time.Time `json:"subscribed_at"`       // Timestamp of when the subscription was created.
	UpdatedAt          time.Time `json:"updated_at"`          // Timestamp of the last modification.
}
