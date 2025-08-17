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

// addressRepository implements the domain.AddressRepository interface.
type addressRepository struct {
	fx.In

	q *query.Query
}

// NewAddressRepository is the constructor for addressRepository.
func NewAddressRepository(db *gorm.DB) repository.AddressRepository {
	return &addressRepository{
		q: query.Use(db),
	}
}

// CreateAddress persists a new address for an owner.
func (repo *addressRepository) CreateAddress(ctx context.Context, address *entity.Address) error {
	addressM := fromAddressDomain(address)

	if err := repo.q.AddressModel.WithContext(ctx).Create(addressM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrPrimaryAddressConflict.WrapMessage("primary address already exists for this owner")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid owner reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required address information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create address")
	}

	// Update the entity with generated values
	address.ID = addressM.ID
	address.CreatedAt = addressM.CreatedAt
	address.UpdatedAt = addressM.UpdatedAt

	return nil
}

// FindAddressByID retrieves an address by its unique ID.
func (repo *addressRepository) FindAddressByID(ctx context.Context, id uuid.UUID) (*entity.Address, error) {
	addressM, err := repo.q.AddressModel.WithContext(ctx).
		Where(repo.q.AddressModel.ID.Eq(id)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrAddressNotFound
		}

		return nil, errors.Wrap(err, "failed to find address by ID")
	}

	return toAddressDomain(addressM), nil
}

// FindAddressesByOwner retrieves all addresses for a specific owner.
func (repo *addressRepository) FindAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType string) ([]*entity.Address, error) {
	addressModels, err := repo.q.AddressModel.WithContext(ctx).
		Where(
			repo.q.AddressModel.OwnerID.Eq(ownerID),
			repo.q.AddressModel.OwnerType.Eq(ownerType),
		).
		Order(repo.q.AddressModel.IsPrimary.Desc(), repo.q.AddressModel.CreatedAt.Asc()).
		Find()

	if err != nil {
		return nil, errors.Wrap(err, "failed to find addresses by owner")
	}

	addresses := make([]*entity.Address, 0, len(addressModels))
	for _, addressM := range addressModels {
		addresses = append(addresses, toAddressDomain(addressM))
	}

	return addresses, nil
}

// FindPrimaryAddressByOwner retrieves the primary address for a specific owner.
func (repo *addressRepository) FindPrimaryAddressByOwner(ctx context.Context, ownerID uuid.UUID, ownerType string) (*entity.Address, error) {
	addressM, err := repo.q.AddressModel.WithContext(ctx).
		Where(
			repo.q.AddressModel.OwnerID.Eq(ownerID),
			repo.q.AddressModel.OwnerType.Eq(ownerType),
			repo.q.AddressModel.IsPrimary.Is(true),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrAddressNotFound
		}

		return nil, errors.Wrap(err, "failed to find primary address by owner")
	}

	return toAddressDomain(addressM), nil
}

// UpdateAddress updates an existing address record.
func (repo *addressRepository) UpdateAddress(ctx context.Context, address *entity.Address) error {
	addressM := fromAddressDomain(address)

	if err := repo.q.AddressModel.WithContext(ctx).Save(addressM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrPrimaryAddressConflict.WrapMessage("primary address already exists for this owner")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("invalid owner reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("missing required address information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to update address")
	}

	// Update the entity with updated timestamp
	address.UpdatedAt = addressM.UpdatedAt

	return nil
}

// DeleteAddress removes an address by its ID.
func (repo *addressRepository) DeleteAddress(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.AddressModel.WithContext(ctx).
		Where(repo.q.AddressModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return errors.Wrap(err, "failed to delete address")
	}

	// If no rows were affected, it means the address was not found.
	if result.RowsAffected == 0 {
		return repository.ErrAddressNotFound
	}

	return nil
}

// --- Mapper Functions ---

// toAddressDomain converts a GORM AddressModel to a domain Address entity.
func toAddressDomain(data *model.AddressModel) *entity.Address {
	if data == nil {
		return nil
	}

	return &entity.Address{
		ID:          data.ID,
		OwnerID:     data.OwnerID,
		OwnerType:   data.OwnerType,
		Label:       data.Label,
		FullAddress: data.FullAddress,
		Latitude:    data.Latitude,
		Longitude:   data.Longitude,
		IsPrimary:   data.IsPrimary,
		CreatedAt:   data.CreatedAt,
		UpdatedAt:   data.UpdatedAt,
	}
}

// fromAddressDomain converts a domain Address entity to a GORM AddressModel.
func fromAddressDomain(data *entity.Address) *model.AddressModel {
	if data == nil {
		return nil
	}

	return &model.AddressModel{
		ID:          data.ID,
		OwnerID:     data.OwnerID,
		OwnerType:   data.OwnerType,
		Label:       data.Label,
		FullAddress: data.FullAddress,
		Latitude:    data.Latitude,
		Longitude:   data.Longitude,
		IsPrimary:   data.IsPrimary,
		CreatedAt:   data.CreatedAt,
		UpdatedAt:   data.UpdatedAt,
	}
}
