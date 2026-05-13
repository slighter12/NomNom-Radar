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
