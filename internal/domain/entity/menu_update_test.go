package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMenuItem_ApplyUpdate_ReplacesMutableFieldsAndPreservesDisplayOrder(t *testing.T) {
	oldDescription := "Old description"
	oldCategoryID := uuid.New()
	oldImageURL := "https://example.com/old.png"
	oldExternalURL := "https://example.com/old"
	item := &MenuItem{
		Name:         "Old Name",
		Description:  &oldDescription,
		CategoryID:   &oldCategoryID,
		Price:        100,
		Currency:     CurrencyTWD,
		PrepMinutes:  5,
		IsAvailable:  true,
		IsPopular:    true,
		DisplayOrder: 4,
		ImageURL:     &oldImageURL,
		ExternalURL:  &oldExternalURL,
	}
	categoryID := uuid.New()
	update := &MenuItemUpdate{
		Name:        "New Name",
		CategoryID:  categoryID,
		Price:       0,
		Currency:    CurrencyTWD,
		PrepMinutes: 0,
		IsAvailable: false,
		IsPopular:   false,
	}
	item.ApplyUpdate(update)

	assert.Equal(t, "New Name", item.Name)
	assert.Nil(t, item.Description)
	assert.Equal(t, categoryID, *item.CategoryID)
	assert.Equal(t, 0, item.Price)
	assert.Equal(t, 0, item.PrepMinutes)
	assert.False(t, item.IsAvailable)
	assert.False(t, item.IsPopular)
	assert.Equal(t, 4, item.DisplayOrder)
	assert.Nil(t, item.ImageURL)
	assert.Nil(t, item.ExternalURL)
}
