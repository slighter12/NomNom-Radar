package entity

import (
	"time"

	"github.com/google/uuid"
)

// LoginAttempt tracks progressive login throttling state for a normalized email key.
type LoginAttempt struct {
	ID            uuid.UUID  `json:"id"`
	AttemptKey    string     `json:"attempt_key"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
	FailedCount   int        `json:"failed_count"`
	LockoutCount  int        `json:"lockout_count"`
	LockedUntil   *time.Time `json:"locked_until,omitempty"`
	LastFailedAt  *time.Time `json:"last_failed_at,omitempty"`
	LastLockoutAt *time.Time `json:"last_lockout_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
