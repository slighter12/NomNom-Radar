// Package entity contains the core business objects of the project.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// Address is the core entity for a physical location.
// It can be associated with different types of owners, like a User or a Merchant.
type Address struct {
	ID          uuid.UUID // The Global Unique Identifier (GUID) for the address.
	OwnerID     uuid.UUID // The ID of the entity that owns this address.
	OwnerType   string    // The type of the owner (e.g., "user_profile", "merchant_profile").
	Label       string    // A user-defined label, e.g., "Home", "Office".
	FullAddress string    // The full, human-readable street address.
	Latitude    float64   // The geographic latitude.
	Longitude   float64   // The geographic longitude.
	IsPrimary   bool      // Indicates if this is the primary address for the owner.
	CreatedAt   time.Time // Timestamp of when this address was created.
	UpdatedAt   time.Time // Timestamp of the last modification.
}
