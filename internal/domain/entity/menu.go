package entity

import (
	"time"

	"github.com/google/uuid"
)

// CurrencyTWD indicates New Taiwan Dollar. Menu prices are stored in minor units;
// for TWD the minor unit is currently the same as whole dollars used by the UI.
const CurrencyTWD = "TWD"

type MenuItem struct {
	ID           uuid.UUID  `json:"id"`
	MerchantID   uuid.UUID  `json:"merchant_id"`
	Name         string     `json:"name"`
	Description  *string    `json:"description"`
	CategoryID   *uuid.UUID `json:"category_id"`
	Price        int        `json:"price"` // Base price stored in minor units before any future promotion or discount rules.
	Currency     string     `json:"currency"`
	PrepMinutes  int        `json:"prep_minutes"`
	IsAvailable  bool       `json:"is_available"`
	IsPopular    bool       `json:"is_popular"`
	DisplayOrder int        `json:"display_order"`
	ImageURL     *string    `json:"image_url"`
	ExternalURL  *string    `json:"external_url"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// MenuItemUpdate describes the full replacement payload for a menu item PUT.
// Nullable fields are intentionally represented as pointers so nil clears the
// corresponding nullable database column.
type MenuItemUpdate struct {
	Name        string
	Description *string
	CategoryID  uuid.UUID
	Price       int
	Currency    string
	PrepMinutes int
	IsAvailable bool
	IsPopular   bool
	ImageURL    *string
	ExternalURL *string
}

// ApplyUpdate applies a full menu item replacement to the entity.
func (item *MenuItem) ApplyUpdate(update *MenuItemUpdate) {
	item.Name = update.Name
	item.Description = update.Description
	categoryID := update.CategoryID
	item.CategoryID = &categoryID
	item.Price = update.Price
	item.Currency = update.Currency
	item.PrepMinutes = update.PrepMinutes
	item.IsAvailable = update.IsAvailable
	item.IsPopular = update.IsPopular
	item.ImageURL = update.ImageURL
	item.ExternalURL = update.ExternalURL
	item.UpdatedAt = time.Now()
}
