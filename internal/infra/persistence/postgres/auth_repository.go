// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"

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
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrUserAlreadyExists.WrapMessage("authentication method already exists")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid user reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required authentication information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create authentication")
	}

	// Update the entity with generated values
	auth.ID = authM.ID
	auth.CreatedAt = authM.CreatedAt

	return nil
}

// FindAuthentication retrieves an authentication record by its provider and provider-specific ID.
func (repo *authRepository) FindAuthentication(ctx context.Context, provider entity.ProviderType, providerUserID string) (*entity.Authentication, error) {
	authM, err := repo.q.AuthenticationModel.WithContext(ctx).
		Where(
			repo.q.AuthenticationModel.Provider.Eq(string(provider)),
			repo.q.AuthenticationModel.ProviderUserID.Eq(providerUserID),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrAuthNotFound
		}

		return nil, errors.WithStack(err)
	}

	return toAuthenticationDomain(authM), nil
}

// FindAuthenticationByUserIDAndProvider finds an authentication method for a specific user and provider.
func (repo *authRepository) FindAuthenticationByUserIDAndProvider(ctx context.Context, userID uuid.UUID, provider entity.ProviderType) (*entity.Authentication, error) {
	authM, err := repo.q.AuthenticationModel.WithContext(ctx).
		Where(
			repo.q.AuthenticationModel.UserID.Eq(userID),
			repo.q.AuthenticationModel.Provider.Eq(string(provider)),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrAuthNotFound
		}

		return nil, errors.WithStack(err)
	}

	return toAuthenticationDomain(authM), nil
}

// UpdateAuthentication updates an existing authentication record.
func (repo *authRepository) UpdateAuthentication(ctx context.Context, auth *entity.Authentication) error {
	authM := fromAuthenticationDomain(auth)

	if err := repo.q.AuthenticationModel.WithContext(ctx).Save(authM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrUserAlreadyExists.WrapMessage("authentication method already exists")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("invalid user reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("missing required authentication information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to update authentication")
	}

	return nil
}

// DeleteAuthentication removes an authentication method by its ID.
func (repo *authRepository) DeleteAuthentication(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.AuthenticationModel.WithContext(ctx).
		Where(repo.q.AuthenticationModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return errors.WithStack(err)
	}

	// If no rows were affected, it means the authentication was not found.
	if result.RowsAffected == 0 {
		return repository.ErrAuthNotFound
	}

	return nil
}

// ListAuthenticationsByUserID returns all authentication methods for a specific user.
func (repo *authRepository) ListAuthenticationsByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Authentication, error) {
	authModels, err := repo.q.AuthenticationModel.WithContext(ctx).
		Where(repo.q.AuthenticationModel.UserID.Eq(userID)).
		Find()

	if err != nil {
		return nil, errors.WithStack(err)
	}

	authentications := make([]*entity.Authentication, 0, len(authModels))
	for _, authM := range authModels {
		authentications = append(authentications, toAuthenticationDomain(authM))
	}

	return authentications, nil
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
		Provider:       entity.ProviderType(data.Provider),
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
		Provider:       string(data.Provider),
		ProviderUserID: data.ProviderUserID,
		PasswordHash:   data.PasswordHash,
	}
}
