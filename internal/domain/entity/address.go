// Package entity contains the core business objects of the project.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// Address is the core entity for a physical location.
// It can be associated with different types of owners, like a User or a Merchant.
type Address struct {
	ID          uuid.UUID `json:"id"`           // The Global Unique Identifier (GUID) for the address.
	OwnerID     uuid.UUID `json:"owner_id"`     // The ID of the entity that owns this address.
	OwnerType   OwnerType `json:"owner_type"`   // The type of the owner (e.g., OwnerTypeUserProfile, OwnerTypeMerchantProfile).
	Label       string    `json:"label"`        // A user-defined label, e.g., "Home", "Office".
	FullAddress string    `json:"full_address"` // The full, human-readable street address.
	Latitude    float64   `json:"latitude"`     // The geographic latitude.
	Longitude   float64   `json:"longitude"`    // The geographic longitude.
	IsPrimary   bool      `json:"is_primary"`   // Indicates if this is the primary address for the owner.
	IsActive    bool      `json:"is_active"`    // Indicates if this location is active for notifications.
	CreatedAt   time.Time `json:"created_at"`   // Timestamp of when this address was created.
	UpdatedAt   time.Time `json:"updated_at"`   // Timestamp of the last modification.
}
