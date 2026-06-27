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
