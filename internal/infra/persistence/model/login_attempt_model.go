package model

import (
	"time"

	"github.com/google/uuid"
)

// LoginAttemptModel mirrors the 'login_attempts' table used for brute-force protection.
type LoginAttemptModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v7()"`
	AttemptKey    string     `gorm:"type:text;not null;uniqueIndex"`
	UserID        *uuid.UUID `gorm:"type:uuid"`
	FailedCount   int        `gorm:"not null;default:0"`
	LockoutCount  int        `gorm:"not null;default:0"`
	LockedUntil   *time.Time `gorm:"type:timestamptz"`
	LastFailedAt  *time.Time `gorm:"type:timestamptz"`
	LastLockoutAt *time.Time `gorm:"type:timestamptz"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName explicitly sets the table name for GORM.
func (LoginAttemptModel) TableName() string {
	return "login_attempts"
}
