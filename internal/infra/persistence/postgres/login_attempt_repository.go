package postgres

import (
	"context"
	"errors"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type loginAttemptRepository struct {
	q *query.Query
}

// NewLoginAttemptRepository is the constructor for loginAttemptRepository.
func NewLoginAttemptRepository(db *gorm.DB) repository.LoginAttemptRepository {
	return &loginAttemptRepository{q: query.Use(db)}
}

func (repo *loginAttemptRepository) FindOrCreateByAttemptKey(
	ctx context.Context,
	attemptKey string,
	userID *uuid.UUID,
) (*entity.LoginAttempt, error) {
	if err := repo.q.LoginAttemptModel.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "attempt_key"}},
			DoNothing: true,
		}).
		Create(&model.LoginAttemptModel{
			AttemptKey: attemptKey,
			UserID:     userID,
		}); err != nil {
		return nil, domainerrors.ErrPersistenceFailed
	}

	attemptModel, err := repo.q.LoginAttemptModel.WithContext(ctx).
		Where(repo.q.LoginAttemptModel.AttemptKey.Eq(attemptKey)).
		Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrPersistenceFailed
		}

		return nil, domainerrors.ErrPersistenceFailed
	}

	return toLoginAttemptDomain(attemptModel), nil
}

func (repo *loginAttemptRepository) IncrementFailedCount(
	ctx context.Context,
	attemptKey string,
	maxAttempts int,
	lockoutPolicy policy.LoginThrottlePolicy,
) (*entity.LoginAttempt, error) {
	var updated *model.LoginAttemptModel
	err := repo.q.Transaction(func(transactionQuery *query.Query) error {
		attemptModel, err := transactionQuery.LoginAttemptModel.WithContext(ctx).
			Clauses(clause.Locking{Strength: rowLockStrengthUpdate}).
			Where(transactionQuery.LoginAttemptModel.AttemptKey.Eq(attemptKey)).
			Take()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainerrors.ErrPersistenceFailed
			}

			return domainerrors.ErrPersistenceFailed
		}

		now := time.Now()
		if attemptModel.LockedUntil != nil && attemptModel.LockedUntil.After(now) {
			updated = attemptModel

			return repository.ErrLoginAttemptLocked
		}

		attemptModel.LastFailedAt = &now
		attemptModel.UpdatedAt = now

		if attemptModel.FailedCount+1 >= maxAttempts {
			lockoutMinutes := lockoutPolicy.LockoutMinutes(attemptModel.LockoutCount)
			lockedUntil := now.Add(time.Duration(lockoutMinutes) * time.Minute)

			attemptModel.LockoutCount++
			attemptModel.LockedUntil = &lockedUntil
			attemptModel.LastLockoutAt = &now
			attemptModel.FailedCount = 0
		} else {
			attemptModel.FailedCount++
		}

		if err := transactionQuery.LoginAttemptModel.WithContext(ctx).Save(attemptModel); err != nil {
			return domainerrors.ErrPersistenceFailed
		}

		updated = attemptModel

		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrLoginAttemptLocked) {
			return toLoginAttemptDomain(updated), err
		}

		return nil, domainerrors.ErrPersistenceFailed
	}

	return toLoginAttemptDomain(updated), nil
}

func (repo *loginAttemptRepository) ResetOnSuccess(ctx context.Context, attemptKey string) error {
	if _, err := repo.q.LoginAttemptModel.WithContext(ctx).
		Where(repo.q.LoginAttemptModel.AttemptKey.Eq(attemptKey)).
		UpdateSimple(
			repo.q.LoginAttemptModel.FailedCount.Value(0),
			repo.q.LoginAttemptModel.LockoutCount.Value(0),
			repo.q.LoginAttemptModel.LockedUntil.Null(),
			repo.q.LoginAttemptModel.LastFailedAt.Null(),
			repo.q.LoginAttemptModel.LastLockoutAt.Null(),
		); err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

func (repo *loginAttemptRepository) ResetForAccountCreation(ctx context.Context, attemptKey string, userID uuid.UUID) error {
	if _, err := repo.q.LoginAttemptModel.WithContext(ctx).
		Where(
			repo.q.LoginAttemptModel.AttemptKey.Eq(attemptKey),
			repo.q.LoginAttemptModel.UserID.IsNull(),
		).
		UpdateSimple(
			repo.q.LoginAttemptModel.UserID.Value(userID),
			repo.q.LoginAttemptModel.FailedCount.Value(0),
			repo.q.LoginAttemptModel.LockoutCount.Value(0),
			repo.q.LoginAttemptModel.LockedUntil.Null(),
			repo.q.LoginAttemptModel.LastFailedAt.Null(),
			repo.q.LoginAttemptModel.LastLockoutAt.Null(),
		); err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

func (repo *loginAttemptRepository) DecayLockoutCounts(ctx context.Context, decayDays int) error {
	cutoff := time.Now().AddDate(0, 0, -decayDays)
	if _, err := repo.q.LoginAttemptModel.WithContext(ctx).
		Where(
			repo.q.LoginAttemptModel.LastLockoutAt.IsNotNull(),
			repo.q.LoginAttemptModel.LastLockoutAt.Lt(cutoff),
			field.Or(
				repo.q.LoginAttemptModel.LastFailedAt.IsNull(),
				repo.q.LoginAttemptModel.LastFailedAt.LteCol(repo.q.LoginAttemptModel.LastLockoutAt),
			),
			repo.q.LoginAttemptModel.LockoutCount.Gt(0),
		).
		UpdateSimple(
			repo.q.LoginAttemptModel.LockoutCount.Value(0),
			repo.q.LoginAttemptModel.LockedUntil.Null(),
			repo.q.LoginAttemptModel.LastLockoutAt.Null(),
		); err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

func toLoginAttemptDomain(data *model.LoginAttemptModel) *entity.LoginAttempt {
	if data == nil {
		return nil
	}

	return &entity.LoginAttempt{
		ID:            data.ID,
		AttemptKey:    data.AttemptKey,
		UserID:        data.UserID,
		FailedCount:   data.FailedCount,
		LockoutCount:  data.LockoutCount,
		LockedUntil:   data.LockedUntil,
		LastFailedAt:  data.LastFailedAt,
		LastLockoutAt: data.LastLockoutAt,
		CreatedAt:     data.CreatedAt,
		UpdatedAt:     data.UpdatedAt,
	}
}
