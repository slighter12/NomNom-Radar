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
	ID              uuid.UUID        // The Global Unique Identifier (GUID) for the user.
	Email           string           // The user's primary contact email, often used as a login identifier.
	Name            string           // The user's display name or real name.
	UserProfile     *UserProfile     // A pointer to the user-specific profile. Will be nil if this person does not have a 'user' role.
	MerchantProfile *MerchantProfile // A pointer to the merchant-specific profile. Will be nil if this person does not have a 'merchant' role.
	CreatedAt       time.Time        // Timestamp of when this user account was created.
	UpdatedAt       time.Time        // Timestamp of the last modification to this user's data.
}

// UserProfile holds data specific to the "regular user" role.
type UserProfile struct {
	UserID                 uuid.UUID // Foreign Key that links this profile to a core User entity.
	DefaultShippingAddress string    // The user's default shipping address for orders.
	LoyaltyPoints          int       // Represents the user's loyalty points in the system.
	UpdatedAt              time.Time // Timestamp of the last modification to this profile.
}

// MerchantProfile holds data specific to the "merchant" role.
type MerchantProfile struct {
	UserID           uuid.UUID // Foreign Key that links this profile to a core User entity.
	StoreName        string    // The merchant's official store name.
	StoreDescription string    // A description of the store and its products.
	BusinessLicense  string    // The merchant's official business license number.
	StoreAddress     string    // The physical address of the merchant's store.
	UpdatedAt        time.Time // Timestamp of the last modification to this profile.
}
