// Package entity contains the core business objects of the project.
package entity

// OwnerType represents the type of entity that can own an address.
type OwnerType string

const (
	// OwnerTypeUserProfile indicates the address belongs to a user profile.
	OwnerTypeUserProfile OwnerType = "user_profile"
	// OwnerTypeMerchantProfile indicates the address belongs to a merchant profile.
	OwnerTypeMerchantProfile OwnerType = "merchant_profile"
)

// String returns the string representation of the OwnerType.
func (o OwnerType) String() string {
	return string(o)
}

// IsValid checks if the OwnerType is a valid value.
func (o OwnerType) IsValid() bool {
	switch o {
	case OwnerTypeUserProfile, OwnerTypeMerchantProfile:
		return true
	default:
		return false
	}
}
