package model

import (
	"time"

	"github.com/google/uuid"
)

// AddressModel is the GORM-specific struct for the 'addresses' table.
type AddressModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OwnerID     uuid.UUID `gorm:"not null;index:idx_addresses_on_owner"`
	OwnerType   string    `gorm:"type:varchar(255);not null;index:idx_addresses_on_owner"`
	Label       string    `gorm:"type:varchar(100);not null"`
	FullAddress string    `gorm:"type:text;not null"`
	Latitude    float64   `gorm:"type:decimal(10,8);not null"`
	Longitude   float64   `gorm:"type:decimal(11,8);not null"`
	IsPrimary   bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName explicitly sets the table name for GORM.
func (AddressModel) TableName() string {
	return "addresses"
}
