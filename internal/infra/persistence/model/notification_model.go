package model

import (
	"time"

	"github.com/google/uuid"
)

// MerchantLocationNotificationModel is the GORM-specific struct for the 'merchant_location_notifications' table.
// It represents a location notification published by a merchant.
type MerchantLocationNotificationModel struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	MerchantID   uuid.UUID  `gorm:"type:uuid;not null;index"`
	AddressID    *uuid.UUID `gorm:"type:uuid"`
	LocationName string     `gorm:"type:text;not null"`
	FullAddress  string     `gorm:"type:text;not null"`
	Latitude     float64    `gorm:"type:decimal(10,8);not null"`
	Longitude    float64    `gorm:"type:decimal(11,8);not null"`
	// Note: location GEOMETRY(POINT, 4326) column exists in database but is not mapped here.
	// It is automatically calculated from Latitude/Longitude via database trigger.
	// Use raw SQL queries with PostGIS functions (ST_Distance, ST_DWithin) for geospatial operations.
	HintMessage string `gorm:"type:text"`
	TotalSent   int    `gorm:"not null;default:0"`
	TotalFailed int    `gorm:"not null;default:0"`
	PublishedAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName explicitly sets the table name for GORM.
func (MerchantLocationNotificationModel) TableName() string {
	return "merchant_location_notifications"
}

// NotificationLogModel is the GORM-specific struct for the 'notification_logs' table.
// It represents a log entry for a single notification sent to a user device.
type NotificationLogModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	NotificationID uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index"`
	DeviceID       uuid.UUID `gorm:"type:uuid;not null;index"`
	Status         string    `gorm:"type:text;not null;default:'sent'"`
	FCMMessageID   string    `gorm:"type:text"`
	ErrorMessage   string    `gorm:"type:text"`
	SentAt         time.Time
}

// TableName explicitly sets the table name for GORM.
func (NotificationLogModel) TableName() string {
	return "notification_logs"
}
