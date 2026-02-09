// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// refreshTokenRepository implements the domain.RefreshTokenRepository interface.
type refreshTokenRepository struct {
	fx.In

	q *query.Query
}

// NewRefreshTokenRepository is the constructor for refreshTokenRepository.
func NewRefreshTokenRepository(db *gorm.DB) repository.RefreshTokenRepository {
	return &refreshTokenRepository{
		q: query.Use(db),
	}
}

// CreateRefreshToken persists a new refresh token, representing a user session.
func (repo *refreshTokenRepository) CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	tokenM := fromRefreshTokenDomain(token)

	if err := repo.q.RefreshTokenModel.WithContext(ctx).Create(tokenM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenInvalid.WrapMessage("refresh token already exists")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid user reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required token information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create refresh token")
	}

	// Update the entity with generated values
	token.ID = tokenM.ID
	token.CreatedAt = tokenM.CreatedAt

	return nil
}

// FindRefreshTokenByHash retrieves a refresh token record by its securely stored hash.
func (repo *refreshTokenRepository) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*entity.RefreshToken, error) {
	tokenM, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.TokenHash.Eq(tokenHash)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrRefreshTokenNotFound
		}

		return nil, errors.WithStack(err)
	}

	token := toRefreshTokenDomain(tokenM)

	// Check if token has expired
	if token.ExpiresAt.Before(time.Now()) {
		return nil, repository.ErrRefreshTokenExpired
	}

	return token, nil
}

// FindRefreshTokenByID retrieves a refresh token record by its unique ID.
func (repo *refreshTokenRepository) FindRefreshTokenByID(ctx context.Context, id uuid.UUID) (*entity.RefreshToken, error) {
	tokenM, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.ID.Eq(id)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrRefreshTokenNotFound
		}

		return nil, errors.WithStack(err)
	}

	token := toRefreshTokenDomain(tokenM)

	// Check if token has expired
	if token.ExpiresAt.Before(time.Now()) {
		return nil, repository.ErrRefreshTokenExpired
	}

	return token, nil
}

// FindRefreshTokensByUserID retrieves all active refresh tokens for a specific user.
func (repo *refreshTokenRepository) FindRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error) {
	now := time.Now()

	tokenModels, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.UserID.Eq(userID),
			repo.q.RefreshTokenModel.ExpiresAt.Gt(now),
		).
		Order(repo.q.RefreshTokenModel.CreatedAt.Desc()).
		Find()

	if err != nil {
		return nil, errors.WithStack(err)
	}

	tokens := make([]*entity.RefreshToken, 0, len(tokenModels))
	for _, tokenM := range tokenModels {
		tokens = append(tokens, toRefreshTokenDomain(tokenM))
	}

	return tokens, nil
}

// UpdateRefreshToken updates an existing refresh token record.
func (repo *refreshTokenRepository) UpdateRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	tokenM := fromRefreshTokenDomain(token)

	if err := repo.q.RefreshTokenModel.WithContext(ctx).Save(tokenM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenInvalid.WrapMessage("refresh token already exists")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("invalid user reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("missing required token information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to update refresh token")
	}

	return nil
}

// DeleteRefreshToken removes a refresh token by its ID, effectively ending a session.
func (repo *refreshTokenRepository) DeleteRefreshToken(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return errors.WithStack(err)
	}

	// If no rows were affected, it means the token was not found.
	if result.RowsAffected == 0 {
		return repository.ErrRefreshTokenNotFound
	}

	return nil
}

// DeleteRefreshTokenByHash deletes a refresh token by its hash, effectively ending a session.
func (repo *refreshTokenRepository) DeleteRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	result, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.TokenHash.Eq(tokenHash)).
		Delete()

	if err != nil {
		return errors.WithStack(err)
	}

	// If no rows were affected, it means the token was not found.
	if result.RowsAffected == 0 {
		return repository.ErrRefreshTokenNotFound
	}

	return nil
}

// DeleteRefreshTokensByUserID removes all refresh tokens for a specific user.
func (repo *refreshTokenRepository) DeleteRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	if _, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.UserID.Eq(userID)).
		Delete(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// DeleteExpiredRefreshTokens removes all expired refresh tokens from the database.
func (repo *refreshTokenRepository) DeleteExpiredRefreshTokens(ctx context.Context) error {
	now := time.Now()

	if _, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.ExpiresAt.Lt(now)).
		Delete(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// CountActiveSessionsByUserID returns the number of active (non-expired) sessions for a user.
func (repo *refreshTokenRepository) CountActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	now := time.Now()

	count, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.UserID.Eq(userID),
			repo.q.RefreshTokenModel.ExpiresAt.Gt(now),
		).
		Count()

	if err != nil {
		return 0, errors.WithStack(err)
	}

	return int(count), nil
}

// --- Mapper Functions ---

// toRefreshTokenDomain converts a GORM RefreshTokenModel to a domain RefreshToken entity.
func toRefreshTokenDomain(data *model.RefreshTokenModel) *entity.RefreshToken {
	if data == nil {
		return nil
	}

	return &entity.RefreshToken{
		ID:        data.ID,
		UserID:    data.UserID,
		TokenHash: data.TokenHash,
		ExpiresAt: data.ExpiresAt,
		CreatedAt: data.CreatedAt,
	}
}

// fromRefreshTokenDomain converts a domain RefreshToken entity to a GORM RefreshTokenModel.
func fromRefreshTokenDomain(data *entity.RefreshToken) *model.RefreshTokenModel {
	if data == nil {
		return nil
	}

	return &model.RefreshTokenModel{
		ID:        data.ID,
		UserID:    data.UserID,
		TokenHash: data.TokenHash,
		ExpiresAt: data.ExpiresAt,
		CreatedAt: data.CreatedAt,
	}
}
