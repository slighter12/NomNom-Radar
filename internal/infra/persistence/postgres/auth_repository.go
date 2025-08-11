// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"
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
func (r *authRepository) CreateAuthentication(ctx context.Context, auth *entity.Authentication) error {
	authM := fromAuthenticationDomain(auth)
	return r.q.AuthenticationModel.WithContext(ctx).Create(authM)
}

// FindAuthentication retrieves an authentication record by its provider and provider-specific ID.
func (r *authRepository) FindAuthentication(ctx context.Context, provider string, providerUserID string) (*entity.Authentication, error) {
	authM, err := r.q.AuthenticationModel.WithContext(ctx).
		Where(
			r.q.AuthenticationModel.Provider.Eq(provider),
			r.q.AuthenticationModel.ProviderUserID.Eq(providerUserID),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrAuthNotFound
		}
		return nil, err
	}
	return toAuthenticationDomain(authM), nil
}

// CreateRefreshToken persists a new refresh token record.
func (r *authRepository) CreateRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	tokenM := fromRefreshTokenDomain(token)
	return r.q.RefreshTokenModel.WithContext(ctx).Create(tokenM)
}

// FindRefreshTokenByHash retrieves a refresh token record by its hash.
func (r *authRepository) FindRefreshTokenByHash(ctx context.Context, hash string) (*entity.RefreshToken, error) {
	tokenM, err := r.q.RefreshTokenModel.WithContext(ctx).
		Where(r.q.RefreshTokenModel.TokenHash.Eq(hash)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrTokenNotFound
		}
		return nil, err
	}
	return toRefreshTokenDomain(tokenM), nil
}

// DeleteRefreshTokenByHash deletes a refresh token record by its hash.
func (r *authRepository) DeleteRefreshTokenByHash(ctx context.Context, hash string) error {
	// We use Exec here because GORM's Delete with a struct requires a primary key.
	// Exec is more suitable for deleting based on other unique columns.
	result, err := r.q.RefreshTokenModel.WithContext(ctx).
		Where(r.q.RefreshTokenModel.TokenHash.Eq(hash)).
		Delete()

	if err != nil {
		return err
	}

	// If no rows were affected, it means the token was not found.
	if result.RowsAffected == 0 {
		return repository.ErrTokenNotFound
	}

	return nil
}

// --- Mapper Functions ---

// toAuthenticationDomain converts a GORM AuthenticationModel to a domain Authentication entity.
func toAuthenticationDomain(m *model.AuthenticationModel) *entity.Authentication {
	if m == nil {
		return nil
	}
	return &entity.Authentication{
		ID:             m.ID,
		UserID:         m.UserID,
		Provider:       m.Provider,
		ProviderUserID: m.ProviderUserID,
		PasswordHash:   m.PasswordHash,
		CreatedAt:      m.CreatedAt,
	}
}

// fromAuthenticationDomain converts a domain Authentication entity to a GORM AuthenticationModel.
func fromAuthenticationDomain(e *entity.Authentication) *model.AuthenticationModel {
	if e == nil {
		return nil
	}
	return &model.AuthenticationModel{
		ID:             e.ID,
		UserID:         e.UserID,
		Provider:       e.Provider,
		ProviderUserID: e.ProviderUserID,
		PasswordHash:   e.PasswordHash,
	}
}

// toRefreshTokenDomain converts a GORM RefreshTokenModel to a domain RefreshToken entity.
func toRefreshTokenDomain(m *model.RefreshTokenModel) *entity.RefreshToken {
	if m == nil {
		return nil
	}
	return &entity.RefreshToken{
		ID:        m.ID,
		UserID:    m.UserID,
		TokenHash: m.TokenHash,
		ExpiresAt: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
	}
}

// fromRefreshTokenDomain converts a domain RefreshToken entity to a GORM RefreshTokenModel.
func fromRefreshTokenDomain(e *entity.RefreshToken) *model.RefreshTokenModel {
	if e == nil {
		return nil
	}
	return &model.RefreshTokenModel{
		ID:        e.ID,
		UserID:    e.UserID,
		TokenHash: e.TokenHash,
		ExpiresAt: e.ExpiresAt,
	}
}
