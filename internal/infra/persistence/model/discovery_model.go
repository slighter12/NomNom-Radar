package model

import (
	"time"

	"github.com/google/uuid"
)

type DiscoveryCategoryModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	Slug         string    `gorm:"type:text;not null;uniqueIndex:discovery_categories_slug_unique"`
	Name         string    `gorm:"type:text;not null"`
	DisplayOrder int       `gorm:"not null"`
	Status       string    `gorm:"type:text;not null;default:active"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Subcategories []*DiscoverySubcategoryModel `gorm:"foreignKey:CategoryID"`
}

func (DiscoveryCategoryModel) TableName() string {
	return "discovery_categories"
}

type DiscoverySubcategoryModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	CategoryID   uuid.UUID `gorm:"type:uuid;not null"`
	Slug         string    `gorm:"type:text;not null;uniqueIndex:discovery_subcategories_slug_unique"`
	Name         string    `gorm:"type:text;not null"`
	DisplayOrder int       `gorm:"not null"`
	Status       string    `gorm:"type:text;not null;default:active"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Category *DiscoveryCategoryModel `gorm:"foreignKey:CategoryID"`
}

func (DiscoverySubcategoryModel) TableName() string {
	return "discovery_subcategories"
}

type HubModel struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	Slug      string     `gorm:"type:text;not null;uniqueIndex:hubs_slug_unique"`
	Name      string     `gorm:"type:text;not null"`
	Type      string     `gorm:"type:text;not null"`
	City      string     `gorm:"type:text;not null"`
	AreaName  string     `gorm:"type:text;not null"`
	StartsAt  *time.Time `gorm:"type:timestamptz"`
	EndsAt    *time.Time `gorm:"type:timestamptz"`
	Status    string     `gorm:"type:text;not null;default:active"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (HubModel) TableName() string {
	return "hubs"
}
