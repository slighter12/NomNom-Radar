package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserMerchantSubscriptionModel is the GORM-specific struct for the 'user_merchant_subscriptions' table.
// It represents a user's subscription to a merchant for location notifications.
type UserMerchantSubscriptionModel struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	UserID             uuid.UUID `gorm:"type:uuid;not null;index"`
	MerchantID         uuid.UUID `gorm:"type:uuid;not null;index"`
	IsActive           bool      `gorm:"not null;default:true"`
	NotificationRadius float64   `gorm:"type:decimal(10,2);not null;default:1000.0"`
	SubscribedAt       time.Time
	UpdatedAt          time.Time
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

// TableName explicitly sets the table name for GORM.
func (UserMerchantSubscriptionModel) TableName() string {
	return "user_merchant_subscriptions"
}
