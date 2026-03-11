package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MenuItemModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	MerchantID   uuid.UUID `gorm:"type:uuid;not null;index:idx_menu_items_merchant_deleted,priority:1;uniqueIndex:idx_menu_items_merchant_display_order_active,priority:1,where:deleted_at IS NULL"`
	Name         string    `gorm:"type:text;not null"`
	Description  *string   `gorm:"type:text"`
	Category     string    `gorm:"type:text;not null"`
	Price        int       `gorm:"not null"` // Base price stored in minor units; promotion-adjusted prices should be modeled separately.
	Currency     string    `gorm:"type:text;not null;default:'TWD'"`
	PrepMinutes  int       `gorm:"not null"`
	IsAvailable  bool      `gorm:"not null;default:true"`
	IsPopular    bool      `gorm:"not null;default:false"`
	DisplayOrder int       `gorm:"not null;uniqueIndex:idx_menu_items_merchant_display_order_active,priority:2,where:deleted_at IS NULL"`
	ImageURL     *string   `gorm:"type:text"`
	ExternalURL  *string   `gorm:"type:text"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_menu_items_merchant_deleted,priority:2"`
}

func (MenuItemModel) TableName() string {
	return "menu_items"
}
