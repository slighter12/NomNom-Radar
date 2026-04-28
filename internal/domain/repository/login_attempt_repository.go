package repository

import (
	"context"
	"errors"

	"radar/internal/domain/entity"
	"radar/internal/domain/policy"

	"github.com/google/uuid"
)

var ErrLoginAttemptLocked = errors.New("login attempt is locked")

// LoginAttemptRepository defines persistence operations for login throttling state.
type LoginAttemptRepository interface {
	FindOrCreateByAttemptKey(ctx context.Context, attemptKey string, userID *uuid.UUID) (*entity.LoginAttempt, error)
	IncrementFailedCount(ctx context.Context, attemptKey string, maxAttempts int, lockoutPolicy policy.LoginThrottlePolicy) (*entity.LoginAttempt, error)
	ResetOnSuccess(ctx context.Context, attemptKey string) error
	ResetForAccountCreation(ctx context.Context, attemptKey string, userID uuid.UUID) error
	DecayLockoutCounts(ctx context.Context, decayDays int) error
}
