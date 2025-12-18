package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthenticationModel mirrors the 'user_authentications' table. UUID columns track provider credentials.
type AuthenticationModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	UserID         uuid.UUID `gorm:"type:uuid;not null"`
	Provider       string    `gorm:"type:text;not null;uniqueIndex:idx_auth_provider_provider_user_id"`
	ProviderUserID string    `gorm:"type:text;not null;uniqueIndex:idx_auth_provider_provider_user_id"`
	PasswordHash   string    `gorm:"type:text"`
	CreatedAt      time.Time
}

// TableName explicitly sets the table name for GORM.
func (AuthenticationModel) TableName() string {
	return "user_authentications"
}

// RefreshTokenModel mirrors the 'refresh_tokens' table. UUID columns align with PostgreSQL schema.
type RefreshTokenModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash string    `gorm:"type:text;unique;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

// TableName explicitly sets the table name for GORM.
func (RefreshTokenModel) TableName() string {
	return "refresh_tokens"
}
