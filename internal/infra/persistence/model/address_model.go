package model

import (
	"time"

	"github.com/google/uuid"
)

// AddressModel is the GORM-specific struct for the 'addresses' table.
//
// This table uses a Nullable FK design to support addresses for both user profiles and merchant profiles.
// Each address belongs to exactly one owner (enforced by CHECK constraint in database).
//
// Relationship Logic:
//   - UserProfileID and MerchantProfileID are both nullable pointers
//   - Exactly one must be non-nil (enforced by database CHECK constraint)
//   - Foreign keys with ON DELETE CASCADE ensure referential integrity
//
// Usage Examples:
//
//	// Create a user address
//	userAddr := &AddressModel{
//	    UserProfileID: &userID,
//	    Label:         "Home",
//	    FullAddress:   "123 Main St",
//	    Latitude:      25.033,
//	    Longitude:     121.565,
//	    IsPrimary:     true,
//	}
//	db.Create(userAddr)
//
//	// Create a merchant address
//	merchantAddr := &AddressModel{
//	    MerchantProfileID: &merchantID,
//	    Label:             "Store A",
//	    FullAddress:       "456 Business Ave",
//	    Latitude:          25.042,
//	    Longitude:         121.543,
//	}
//	db.Create(merchantAddr)
//
//	// Query all addresses for a user profile
//	var userAddresses []AddressModel
//	db.Where("user_profile_id = ?", userID).Find(&userAddresses)
//
//	// Query all addresses for a merchant profile
//	var merchantAddresses []AddressModel
//	db.Where("merchant_profile_id = ?", merchantID).Find(&merchantAddresses)
//
//	// Query with preload from UserProfileModel
//	var userProfile UserProfileModel
//	db.Preload("Addresses").First(&userProfile, userID)
//
//	// Query with preload from MerchantProfileModel
//	var merchantProfile MerchantProfileModel
//	db.Preload("Addresses").First(&merchantProfile, merchantID)
//
//	// Find primary address for a user
//	var primaryAddr AddressModel
//	db.Where("user_profile_id = ? AND is_primary = ?", userID, true).First(&primaryAddr)
//
//	// Calculate distance between user and merchant addresses (PostGIS)
//	// SELECT ua.*, ST_Distance(ua.location, ma.location) as distance
//	// FROM addresses ua
//	// JOIN addresses ma ON ma.merchant_profile_id = ?
//	// WHERE ua.user_profile_id = ?
//	//   AND ua.deleted_at IS NULL
//	//   AND ST_DWithin(ua.location, ma.location, 1000);
//
// Database Constraints:
//   - CHECK: (user_profile_id IS NOT NULL)::int + (merchant_profile_id IS NOT NULL)::int = 1
//   - UNIQUE: Only one primary address per owner (partial index with WHERE is_primary = TRUE)
//   - FK: user_profile_id REFERENCES user_profiles(user_id) ON DELETE CASCADE
//   - FK: merchant_profile_id REFERENCES merchant_profiles(user_id) ON DELETE CASCADE
type AddressModel struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	UserProfileID     *uuid.UUID `gorm:"type:uuid;index:idx_addresses_user_profile"`
	MerchantProfileID *uuid.UUID `gorm:"type:uuid;index:idx_addresses_merchant_profile"`
	Label             string     `gorm:"type:varchar(100);not null"`
	FullAddress       string     `gorm:"type:text;not null"`
	Latitude          float64    `gorm:"type:decimal(10,8);not null"`
	Longitude         float64    `gorm:"type:decimal(11,8);not null"`
	// Note: location GEOMETRY(POINT, 4326) column exists in database but is not mapped here.
	// It is automatically calculated from Latitude/Longitude via database trigger.
	// Use raw SQL queries with PostGIS functions (ST_Distance, ST_DWithin) for geospatial operations.
	IsPrimary bool `gorm:"not null;default:false"`
	IsActive  bool `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
}

// IsUserAddress returns true if this address belongs to a user profile
func (a *AddressModel) IsUserAddress() bool {
	return a.UserProfileID != nil
}

// IsMerchantAddress returns true if this address belongs to a merchant profile
func (a *AddressModel) IsMerchantAddress() bool {
	return a.MerchantProfileID != nil
}

// TableName explicitly sets the table name for GORM.
func (AddressModel) TableName() string {
	return "addresses"
}
