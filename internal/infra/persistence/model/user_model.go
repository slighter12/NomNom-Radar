package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserModel mirrors the 'users' table. PostgreSQL generates UUIDs via uuid_generate_v7().
// It is an exported type so it can be used by the GORM Gen tool from other packages.
type UserModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	Email     string    `gorm:"type:citext;not null;uniqueIndex:idx_users_email_active,where:deleted_at IS NULL"`
	Name      string    `gorm:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	UserProfile     *UserProfileModel     `gorm:"foreignKey:UserID"`
	MerchantProfile *MerchantProfileModel `gorm:"foreignKey:UserID"`
	Authentications []AuthenticationModel `gorm:"foreignKey:UserID"`
	RefreshTokens   []RefreshTokenModel   `gorm:"foreignKey:UserID"`
}

// TableName explicitly sets the table name for GORM.
func (UserModel) TableName() string {
	return "users"
}

// UserProfileModel mirrors the 'user_profiles' table. UserID references users.id (UUID).
type UserProfileModel struct {
	UserID        uuid.UUID       `gorm:"primaryKey"`
	Addresses     []*AddressModel `gorm:"foreignKey:UserProfileID"`
	LoyaltyPoints int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName explicitly sets the table name for GORM.
func (UserProfileModel) TableName() string {
	return "user_profiles"
}

// MerchantProfileModel mirrors the 'merchant_profiles' table. UserID references users.id (UUID).
type MerchantProfileModel struct {
	UserID                    uuid.UUID       `gorm:"primaryKey"`
	Addresses                 []*AddressModel `gorm:"foreignKey:MerchantProfileID"`
	StoreName                 string          `gorm:"type:text;not null"`
	StoreDescription          string          `gorm:"type:text"`
	BusinessLicense           *string         `gorm:"type:text;uniqueIndex:idx_merchant_profiles_business_license_active,where:deleted_at IS NULL AND business_license IS NOT NULL"`
	VerificationStatus        string          `gorm:"type:text;not null;default:unverified"`
	BusinessLicenseVerifiedAt *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	DeletedAt                 gorm.DeletedAt `gorm:"index:idx_merchant_profiles_deleted_at"`
}

// TableName explicitly sets the table name for GORM.
func (MerchantProfileModel) TableName() string {
	return "merchant_profiles"
}
