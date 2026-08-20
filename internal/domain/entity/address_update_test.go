package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddressUpdate_HasChanges(t *testing.T) {
	assert.False(t, (AddressUpdate{}).HasChanges())

	label := ""
	assert.True(t, (AddressUpdate{Label: &label}).HasChanges())
}

func TestAddress_ApplyUpdate_PreservesUnspecifiedFields(t *testing.T) {
	previousUpdatedAt := time.Now().Add(-time.Minute)
	address := &Address{
		Label:       "Home",
		FullAddress: "1 Main Street",
		Latitude:    25.0,
		Longitude:   121.0,
		IsPrimary:   true,
		IsActive:    true,
		UpdatedAt:   previousUpdatedAt,
	}

	clearedLabel := ""
	deactivated := false
	address.ApplyUpdate(AddressUpdate{
		Label:    &clearedLabel,
		IsActive: &deactivated,
	})

	assert.Equal(t, "", address.Label)
	assert.False(t, address.IsActive)
	assert.Equal(t, "1 Main Street", address.FullAddress)
	assert.Equal(t, 25.0, address.Latitude)
	assert.Equal(t, 121.0, address.Longitude)
	assert.True(t, address.IsPrimary)
	assert.True(t, address.UpdatedAt.After(previousUpdatedAt))
}
