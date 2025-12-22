// Package entity contains the core business objects of the project.
package entity

import "slices"

// Role represents the type of role a user can have in the system.
type Role string

const (
	// RoleUser indicates a regular user role.
	RoleUser Role = "user"
	// RoleMerchant indicates a merchant role.
	RoleMerchant Role = "merchant"
)

// String returns the string representation of the Role.
func (r Role) String() string {
	return string(r)
}

// IsValid checks if the Role is a valid value.
func (r Role) IsValid() bool {
	switch r {
	case RoleUser, RoleMerchant:
		return true
	default:
		return false
	}
}

// Roles is a slice of Role for convenience.
type Roles []Role

// Contains checks if the roles slice contains a specific role.
func (rs Roles) Contains(role Role) bool {
	return slices.Contains(rs, role)
}

// ToStrings converts Roles to []string for JWT compatibility.
func (rs Roles) ToStrings() []string {
	result := make([]string, len(rs))
	for i, r := range rs {
		result[i] = r.String()
	}

	return result
}

// RolesFromStrings converts []string to Roles, filtering out invalid role strings.
func RolesFromStrings(ss []string) Roles {
	result := make(Roles, 0, len(ss))
	for _, s := range ss {
		role := Role(s)
		if role.IsValid() {
			result = append(result, role)
		}
	}

	return result
}
