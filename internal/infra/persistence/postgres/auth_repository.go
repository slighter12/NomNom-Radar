// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/pkg/errors"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// authRepository implements the domain.AuthRepository interface.
type authRepository struct {
	fx.In

	q *query.Query
}

// NewAuthRepository is the constructor for authRepository.
func NewAuthRepository(db *gorm.DB) repository.AuthRepository {
	return &authRepository{
		q: query.Use(db),
	}
}

// CreateAuthentication persists a new authentication method record.
func (repo *authRepository) CreateAuthentication(ctx context.Context, auth *entity.Authentication) error {
	authM := fromAuthenticationDomain(auth)

	if err := repo.q.AuthenticationModel.WithContext(ctx).Create(authM); err != nil {
		return errors.Wrap(err, "failed to create authentication")
	}

	return nil
}

// FindAuthentication retrieves an authentication record by its provider and provider-specific ID.
func (repo *authRepository) FindAuthentication(ctx context.Context, provider string, providerUserID string) (*entity.Authentication, error) {
	authM, err := repo.q.AuthenticationModel.WithContext(ctx).
		Where(
			repo.q.AuthenticationModel.Provider.Eq(provider),
			repo.q.AuthenticationModel.ProviderUserID.Eq(providerUserID),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrAuthNotFound
		}

		return nil, errors.Wrap(err, "failed to find authentication")
	}

	return toAuthenticationDomain(authM), nil
}

// CreateRefreshToken persists a new refresh token record.
func (repo *authRepository) CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	tokenM := fromRefreshTokenDomain(token)

	if err := repo.q.RefreshTokenModel.WithContext(ctx).Create(tokenM); err != nil {
		return errors.Wrap(err, "failed to create refresh token")
	}

	return nil
}

// FindRefreshTokenByHash retrieves a refresh token record by its hash.
func (repo *authRepository) FindRefreshTokenByHash(ctx context.Context, hash string) (*entity.RefreshToken, error) {
	tokenM, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.TokenHash.Eq(hash)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrTokenNotFound
		}

		return nil, errors.Wrap(err, "failed to find refresh token by hash")
	}

	return toRefreshTokenDomain(tokenM), nil
}

// DeleteRefreshTokenByHash deletes a refresh token record by its hash.
func (repo *authRepository) DeleteRefreshTokenByHash(ctx context.Context, hash string) error {
	// We use Exec here because GORM's Delete with a struct requires a primary key.
	// Exec is more suitable for deleting based on other unique columns.
	result, err := repo.q.RefreshTokenModel.WithContext(ctx).
		Where(repo.q.RefreshTokenModel.TokenHash.Eq(hash)).
		Delete()

	if err != nil {
		return errors.Wrap(err, "failed to delete refresh token by hash")
	}

	// If no rows were affected, it means the token was not found.
	if result.RowsAffected == 0 {
		return repository.ErrTokenNotFound
	}

	return nil
}

// --- Mapper Functions ---

// toAuthenticationDomain converts a GORM AuthenticationModel to a domain Authentication entity.
func toAuthenticationDomain(data *model.AuthenticationModel) *entity.Authentication {
	if data == nil {
		return nil
	}

	return &entity.Authentication{
		ID:             data.ID,
		UserID:         data.UserID,
		Provider:       data.Provider,
		ProviderUserID: data.ProviderUserID,
		PasswordHash:   data.PasswordHash,
		CreatedAt:      data.CreatedAt,
	}
}

// fromAuthenticationDomain converts a domain Authentication entity to a GORM AuthenticationModel.
func fromAuthenticationDomain(data *entity.Authentication) *model.AuthenticationModel {
	if data == nil {
		return nil
	}

	return &model.AuthenticationModel{
		ID:             data.ID,
		UserID:         data.UserID,
		Provider:       data.Provider,
		ProviderUserID: data.ProviderUserID,
		PasswordHash:   data.PasswordHash,
	}
}

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
	}
}
