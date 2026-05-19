package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// LoginAttemptRepository defines persistence operations for login throttling state.
type LoginAttemptRepository interface {
	FindOrCreateByAttemptKey(ctx context.Context, attemptKey string, userID *uuid.UUID) (*entity.LoginAttempt, error)
	FindOrCreateByAttemptKeyForUpdate(ctx context.Context, attemptKey string, userID *uuid.UUID) (*entity.LoginAttempt, error)
	Save(ctx context.Context, attempt *entity.LoginAttempt) error
	ResetOnSuccess(ctx context.Context, attemptKey string) error
	ResetForAccountCreation(ctx context.Context, attemptKey string, userID uuid.UUID) error
	DecayLockoutCounts(ctx context.Context, decayDays int) error
}
