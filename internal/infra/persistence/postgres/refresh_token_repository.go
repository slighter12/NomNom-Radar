package postgres

import (
	"context"
	"errors"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// refreshTokenRepository implements the domain.RefreshTokenRepository interface.
type refreshTokenRepository struct {
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
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenAlreadyExists
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenCreateFailed
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenCreateFailed
		}

		return domainerrors.ErrPersistenceFailed
	}

	// Update the entity with generated values
	token.ID = tokenM.ID
	token.CreatedAt = tokenM.CreatedAt

	return nil
}

// FindRefreshTokenByHash retrieves a refresh token record by its securely stored hash.
func (repo *refreshTokenRepository) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*entity.RefreshToken, error) {
	tokenM, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.TokenHash.Eq(tokenHash),
			repo.q.RefreshTokenModel.IsRevoked.Is(false),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrRefreshTokenNotFound
		}

		return nil, domainerrors.ErrPersistenceFailed
	}

	token := toRefreshTokenDomain(tokenM)

	// Check if token has expired
	if token.ExpiresAt.Before(time.Now()) {
		return nil, domainerrors.ErrRefreshTokenExpired
	}

	return token, nil
}

// FindRefreshTokenByHashIncludingRevoked retrieves a refresh token record by hash, including revoked tokens.
func (repo *refreshTokenRepository) FindRefreshTokenByHashIncludingRevoked(
	ctx context.Context,
	tokenHash string,
) (*entity.RefreshToken, error) {
	tokenM, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.TokenHash.Eq(tokenHash)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrRefreshTokenNotFound
		}

		return nil, domainerrors.ErrPersistenceFailed
	}

	return toRefreshTokenDomain(tokenM), nil
}

// FindRefreshTokenByID retrieves a refresh token record by its unique ID.
func (repo *refreshTokenRepository) FindRefreshTokenByID(ctx context.Context, id uuid.UUID) (*entity.RefreshToken, error) {
	tokenM, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.ID.Eq(id)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrRefreshTokenNotFound
		}

		return nil, domainerrors.ErrPersistenceFailed
	}

	token := toRefreshTokenDomain(tokenM)

	// Check if token has expired
	if token.ExpiresAt.Before(time.Now()) {
		return nil, domainerrors.ErrRefreshTokenExpired
	}

	return token, nil
}

// FindRefreshTokensByUserID retrieves all active refresh tokens for a specific user.
func (repo *refreshTokenRepository) FindRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error) {
	now := time.Now()

	tokenModels, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.UserID.Eq(userID),
			repo.q.RefreshTokenModel.IsRevoked.Is(false),
			repo.q.RefreshTokenModel.ExpiresAt.Gt(now),
		).
		Order(repo.q.RefreshTokenModel.CreatedAt.Desc()).
		Find()

	if err != nil {
		return nil, domainerrors.ErrPersistenceFailed
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
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenAlreadyExists
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenUpdateFailed
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrRefreshTokenUpdateFailed
		}

		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

// DeleteRefreshToken removes a refresh token by its ID, effectively ending a session.
func (repo *refreshTokenRepository) DeleteRefreshToken(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	// If no rows were affected, it means the token was not found.
	if result.RowsAffected == 0 {
		return domainerrors.ErrRefreshTokenNotFound
	}

	return nil
}

// DeleteRefreshTokenByHash deletes a refresh token by its hash, effectively ending a session.
func (repo *refreshTokenRepository) DeleteRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	result, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.TokenHash.Eq(tokenHash)).
		Delete()

	if err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	// If no rows were affected, it means the token was not found.
	if result.RowsAffected == 0 {
		return domainerrors.ErrRefreshTokenNotFound
	}

	return nil
}

// DeleteRefreshTokensByUserID removes all refresh tokens for a specific user.
func (repo *refreshTokenRepository) DeleteRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	if _, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.UserID.Eq(userID)).
		Delete(); err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

// DeleteExpiredRefreshTokens removes all expired refresh tokens from the database.
func (repo *refreshTokenRepository) DeleteExpiredRefreshTokens(ctx context.Context, revokedRetentionDays int) error {
	now := time.Now()
	revokedCutoff := now.AddDate(0, 0, -revokedRetentionDays)

	if err := deleteExpiredRefreshTokensQuery(repo.q.RefreshTokenModel.WithContext(ctx).UnderlyingDB(), now, revokedCutoff).Error; err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

func deleteExpiredRefreshTokensQuery(db *gorm.DB, now, revokedCutoff time.Time) *gorm.DB {
	return db.
		Where("expires_at < ? OR (is_revoked = ? AND created_at < ?)", now, true, revokedCutoff).
		Delete(&model.RefreshTokenModel{})
}

// RevokeTokenFamily marks all tokens in a family as revoked.
func (repo *refreshTokenRepository) RevokeTokenFamily(ctx context.Context, familyID uuid.UUID) error {
	if _, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.FamilyID.Eq(familyID),
			repo.q.RefreshTokenModel.IsRevoked.Is(false),
		).
		Update(repo.q.RefreshTokenModel.IsRevoked, true); err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

// RevokeTokenFamiliesByUserID marks all refresh tokens for a user as revoked.
func (repo *refreshTokenRepository) RevokeTokenFamiliesByUserID(ctx context.Context, userID uuid.UUID) error {
	if _, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.UserID.Eq(userID),
			repo.q.RefreshTokenModel.IsRevoked.Is(false),
		).
		Update(repo.q.RefreshTokenModel.IsRevoked, true); err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}

// CountActiveSessionsByUserID returns the number of active (non-expired) sessions for a user.
func (repo *refreshTokenRepository) CountActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	now := time.Now()

	count, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(
			repo.q.RefreshTokenModel.UserID.Eq(userID),
			repo.q.RefreshTokenModel.IsRevoked.Is(false),
			repo.q.RefreshTokenModel.ExpiresAt.Gt(now),
		).
		Count()

	if err != nil {
		return 0, domainerrors.ErrPersistenceFailed
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
		ID:         data.ID,
		UserID:     data.UserID,
		TokenHash:  data.TokenHash,
		FamilyID:   data.FamilyID,
		IsRevoked:  data.IsRevoked,
		ReplacedBy: data.ReplacedBy,
		ExpiresAt:  data.ExpiresAt,
		CreatedAt:  data.CreatedAt,
	}
}

// fromRefreshTokenDomain converts a domain RefreshToken entity to a GORM RefreshTokenModel.
func fromRefreshTokenDomain(data *entity.RefreshToken) *model.RefreshTokenModel {
	if data == nil {
		return nil
	}

	return &model.RefreshTokenModel{
		ID:         data.ID,
		UserID:     data.UserID,
		TokenHash:  data.TokenHash,
		FamilyID:   data.FamilyID,
		IsRevoked:  data.IsRevoked,
		ReplacedBy: data.ReplacedBy,
		ExpiresAt:  data.ExpiresAt,
		CreatedAt:  data.CreatedAt,
	}
}
