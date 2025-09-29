package model

import (
	"time"

	"github.com/google/uuid"
)

// UserModel mirrors the 'users' table. PostgreSQL generates UUIDs via uuid_generate_v4().
// It is an exported type so it can be used by the GORM Gen tool from other packages.
type UserModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Email     string    `gorm:"type:varchar(255);unique;not null"`
	Name      string    `gorm:"type:varchar(100)"`
	CreatedAt time.Time
	UpdatedAt time.Time

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
	Addresses     []*AddressModel `gorm:"polymorphic:Owner;"`
	LoyaltyPoints int
	UpdatedAt     time.Time
}

// TableName explicitly sets the table name for GORM.
func (UserProfileModel) TableName() string {
	return "user_profiles"
}

// MerchantProfileModel mirrors the 'merchant_profiles' table. UserID references users.id (UUID).
type MerchantProfileModel struct {
	UserID           uuid.UUID       `gorm:"primaryKey"`
	Addresses        []*AddressModel `gorm:"polymorphic:Owner;"`
	StoreName        string          `gorm:"type:varchar(100);not null"`
	StoreDescription string          `gorm:"type:text"`
	BusinessLicense  string          `gorm:"type:varchar(255);not null;unique"`
	UpdatedAt        time.Time
}

// TableName explicitly sets the table name for GORM.
func (MerchantProfileModel) TableName() string {
	return "merchant_profiles"
}
