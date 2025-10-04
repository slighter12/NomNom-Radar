package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserDeviceModel is the GORM-specific struct for the 'user_devices' table.
// It represents a user's device registered for push notifications.
type UserDeviceModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	FCMToken  string    `gorm:"type:varchar(255);not null"`
	DeviceID  string    `gorm:"type:varchar(255);not null"`
	Platform  string    `gorm:"type:varchar(50);not null"`
	IsActive  bool      `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName explicitly sets the table name for GORM.
func (UserDeviceModel) TableName() string {
	return "user_devices"
}
