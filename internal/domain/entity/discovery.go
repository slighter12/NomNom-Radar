package entity

import (
	"time"

	"github.com/google/uuid"
)

type DiscoveryStatus string

const (
	DiscoveryStatusActive   DiscoveryStatus = "active"
	DiscoveryStatusInactive DiscoveryStatus = "inactive"
)

type HubType string

const (
	HubTypeMarket      HubType = "market"
	HubTypeEvent       HubType = "event"
	HubTypeTourismArea HubType = "tourism_area"
	HubTypeTransitArea HubType = "transit_area"
	HubTypeOther       HubType = "other"
)

type DiscoveryCategory struct {
	ID           uuid.UUID       `json:"id"`
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	DisplayOrder int             `json:"display_order"`
	Status       DiscoveryStatus `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type DiscoverySubcategory struct {
	ID           uuid.UUID       `json:"id"`
	CategoryID   uuid.UUID       `json:"category_id"`
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	DisplayOrder int             `json:"display_order"`
	Status       DiscoveryStatus `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type Hub struct {
	ID        uuid.UUID       `json:"id"`
	Slug      string          `json:"slug"`
	Name      string          `json:"name"`
	Type      HubType         `json:"type"`
	City      string          `json:"city"`
	AreaName  string          `json:"area_name"`
	StartsAt  *time.Time      `json:"starts_at,omitempty"`
	EndsAt    *time.Time      `json:"ends_at,omitempty"`
	Status    DiscoveryStatus `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type PublicDiscoveryCategorySummary struct {
	ID           uuid.UUID `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
}

type PublicDiscoverySubcategorySummary struct {
	ID           uuid.UUID `json:"id"`
	CategoryID   uuid.UUID `json:"category_id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
}

type PublicHubSummary struct {
	ID       uuid.UUID  `json:"id"`
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	Type     HubType    `json:"type"`
	City     string     `json:"city"`
	AreaName string     `json:"area_name"`
	StartsAt *time.Time `json:"starts_at,omitempty"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

type PublicMerchantLocationSummary struct {
	ID          uuid.UUID `json:"id"`
	Label       string    `json:"label"`
	FullAddress string    `json:"full_address"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
}

type PublicMerchantSearchItem struct {
	MerchantID           uuid.UUID                          `json:"merchant_id"`
	StoreName            string                             `json:"store_name"`
	StoreDescription     string                             `json:"store_description"`
	DiscoveryCategory    *PublicDiscoveryCategorySummary    `json:"discovery_category"`
	DiscoverySubcategory *PublicDiscoverySubcategorySummary `json:"discovery_subcategory"`
	ActiveHub            *PublicHubSummary                  `json:"active_hub,omitempty"`
	PrimaryLocation      *PublicMerchantLocationSummary     `json:"primary_location"`
	DistanceMeters       *float64                           `json:"distance_meters,omitempty"`
}
