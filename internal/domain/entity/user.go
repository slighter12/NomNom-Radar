// Package entity contains the core business objects of the project,
// each representing a unique, identifiable concept within the domain.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// User is the core entity in the system, representing a unique "person" or "account".
// It contains only the most fundamental identity information shared across all roles.
type User struct {
	ID              uuid.UUID        `json:"id"`                         // The Global Unique Identifier (GUID) for the user.
	Email           string           `json:"email"`                      // The user's primary contact email, often used as a login identifier.
	Name            string           `json:"name"`                       // The user's display name or real name.
	UserProfile     *UserProfile     `json:"user_profile,omitempty"`     // A pointer to the user-specific profile. Will be nil if this person does not have a 'user' role.
	MerchantProfile *MerchantProfile `json:"merchant_profile,omitempty"` // A pointer to the merchant-specific profile. Will be nil if this person does not have a 'merchant' role.
	CreatedAt       time.Time        `json:"created_at"`                 // Timestamp of when this user account was created.
	UpdatedAt       time.Time        `json:"updated_at"`                 // Timestamp of the last modification to this user's data.
}

// UserProfile holds data specific to the "regular user" role.
type UserProfile struct {
	UserID        uuid.UUID  `json:"user_id"`        // Foreign Key that links this profile to a core User entity.
	Addresses     []*Address `json:"addresses"`      // A user can have multiple addresses.
	LoyaltyPoints int        `json:"loyalty_points"` // Represents the user's loyalty points in the system.
	UpdatedAt     time.Time  `json:"updated_at"`     // Timestamp of the last modification to this profile.
}

// MerchantProfile holds data specific to the "merchant" role.
type MerchantProfile struct {
	UserID           uuid.UUID  `json:"user_id"`           // Foreign Key that links this profile to a core User entity.
	Addresses        []*Address `json:"addresses"`         // A merchant can also have multiple addresses.
	StoreName        string     `json:"store_name"`        // The merchant's official store name.
	StoreDescription string     `json:"store_description"` // A description of the store and its products.
	BusinessLicense  string     `json:"business_license"`  // The merchant's official business license number.
	UpdatedAt        time.Time  `json:"updated_at"`        // Timestamp of the last modification to this profile.
}
