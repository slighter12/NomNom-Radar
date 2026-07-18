package postgres

import (
	"context"
	"errors"
	"fmt"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"
	"radar/internal/platform/observability"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type addressRepository struct {
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
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrPrimaryAddressConflict
		}
		if isForeignKeyConstraintViolation(err) {
			return withSourceStack(domainerrors.ErrAddressCreateFailed)
		}
		if isNotNullConstraintViolation(err) {
			return withSourceStack(domainerrors.ErrAddressCreateFailed)
		}

		return withSourceStack(domainerrors.ErrPersistenceFailed)
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
			return nil, domainerrors.ErrAddressNotFound
		}

		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return toAddressDomain(addressM), nil
}

// FindAddressesByOwner retrieves all addresses for a specific owner (excluding soft-deleted).
func (repo *addressRepository) FindAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) ([]*entity.Address, error) {
	query := repo.q.AddressModel.WithContext(ctx)

	// Apply owner filter based on owner type
	switch ownerType {
	case entity.OwnerTypeUserProfile:
		query = query.Where(repo.q.AddressModel.UserProfileID.Eq(ownerID))
	case entity.OwnerTypeMerchantProfile:
		query = query.Where(repo.q.AddressModel.MerchantProfileID.Eq(ownerID))
	default:
		return nil, observability.WithSourceStack(fmt.Errorf("unsupported owner type: %s", ownerType))
	}

	// Filter out soft-deleted addresses
	query = query.Where(repo.q.AddressModel.DeletedAt.IsNull())

	addressModels, err := query.
		Order(repo.q.AddressModel.IsPrimary.Desc(), repo.q.AddressModel.CreatedAt.Asc()).
		Find()

	if err != nil {
		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	addresses := make([]*entity.Address, 0, len(addressModels))
	for _, addressM := range addressModels {
		addresses = append(addresses, toAddressDomain(addressM))
	}

	return addresses, nil
}

// FindPrimaryAddressByOwner retrieves the primary address for a specific owner.
func (repo *addressRepository) FindPrimaryAddressByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) (*entity.Address, error) {
	query := repo.q.AddressModel.WithContext(ctx).
		Where(repo.q.AddressModel.IsPrimary.Is(true))

	// Apply owner filter based on owner type
	switch ownerType {
	case entity.OwnerTypeUserProfile:
		query = query.Where(repo.q.AddressModel.UserProfileID.Eq(ownerID))
	case entity.OwnerTypeMerchantProfile:
		query = query.Where(repo.q.AddressModel.MerchantProfileID.Eq(ownerID))
	default:
		return nil, observability.WithSourceStack(fmt.Errorf("unsupported owner type: %s", ownerType))
	}

	addressM, err := query.First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrAddressNotFound
		}

		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return toAddressDomain(addressM), nil
}

// UpdateAddress updates an existing address record.
func (repo *addressRepository) UpdateAddress(ctx context.Context, address *entity.Address) error {
	addressM := fromAddressDomain(address)

	if err := repo.q.AddressModel.WithContext(ctx).Save(addressM); err != nil {
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrPrimaryAddressConflict
		}
		if isForeignKeyConstraintViolation(err) {
			return withSourceStack(domainerrors.ErrAddressUpdateFailed)
		}
		if isNotNullConstraintViolation(err) {
			return withSourceStack(domainerrors.ErrAddressUpdateFailed)
		}

		return withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	// Update the entity with updated timestamp
	address.UpdatedAt = addressM.UpdatedAt

	return nil
}

// DeleteAddress removes an address by its ID (soft delete).
func (repo *addressRepository) DeleteAddress(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.AddressModel.WithContext(ctx).
		Where(repo.q.AddressModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	// If no rows were affected, it means the address was not found.
	if result.RowsAffected == 0 {
		return domainerrors.ErrAddressNotFound
	}

	return nil
}

// CountAddressesByOwner returns the total count of addresses for a specific owner (excluding soft-deleted).
func (repo *addressRepository) CountAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) (int64, error) {
	query := repo.q.AddressModel.WithContext(ctx)

	// Apply owner filter based on owner type
	switch ownerType {
	case entity.OwnerTypeUserProfile:
		query = query.Where(repo.q.AddressModel.UserProfileID.Eq(ownerID))
	case entity.OwnerTypeMerchantProfile:
		query = query.Where(repo.q.AddressModel.MerchantProfileID.Eq(ownerID))
	default:
		return 0, observability.WithSourceStack(fmt.Errorf("unsupported owner type: %s", ownerType))
	}

	// Filter out soft-deleted addresses
	query = query.Where(repo.q.AddressModel.DeletedAt.IsNull())

	count, err := query.Count()
	if err != nil {
		return 0, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return count, nil
}

// FindActiveAddressesByOwner retrieves all active addresses (IsActive=true and not soft-deleted) for a specific owner.
func (repo *addressRepository) FindActiveAddressesByOwner(ctx context.Context, ownerID uuid.UUID, ownerType entity.OwnerType) ([]*entity.Address, error) {
	query := repo.q.AddressModel.WithContext(ctx)

	// Apply owner filter based on owner type
	switch ownerType {
	case entity.OwnerTypeUserProfile:
		query = query.Where(repo.q.AddressModel.UserProfileID.Eq(ownerID))
	case entity.OwnerTypeMerchantProfile:
		query = query.Where(repo.q.AddressModel.MerchantProfileID.Eq(ownerID))
	default:
		return nil, observability.WithSourceStack(fmt.Errorf("unsupported owner type: %s", ownerType))
	}

	// Filter for active addresses and exclude soft-deleted
	query = query.Where(
		repo.q.AddressModel.IsActive.Is(true),
		repo.q.AddressModel.DeletedAt.IsNull(),
	)

	addressModels, err := query.
		Order(repo.q.AddressModel.IsPrimary.Desc(), repo.q.AddressModel.CreatedAt.Asc()).
		Find()

	if err != nil {
		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	addresses := make([]*entity.Address, 0, len(addressModels))
	for _, addressM := range addressModels {
		addresses = append(addresses, toAddressDomain(addressM))
	}

	return addresses, nil
}

// --- Mapper Functions ---

// toAddressDomain converts a GORM AddressModel to a domain Address entity.
func toAddressDomain(data *model.AddressModel) *entity.Address {
	if data == nil {
		return nil
	}

	// Determine owner ID and type from nullable FK fields
	var ownerID uuid.UUID
	var ownerType entity.OwnerType

	if data.UserProfileID != nil {
		ownerID = *data.UserProfileID
		ownerType = entity.OwnerTypeUserProfile
	} else if data.MerchantProfileID != nil {
		ownerID = *data.MerchantProfileID
		ownerType = entity.OwnerTypeMerchantProfile
	}

	return &entity.Address{
		ID:          data.ID,
		OwnerID:     ownerID,
		OwnerType:   ownerType,
		Label:       data.Label,
		FullAddress: data.FullAddress,
		Latitude:    data.Latitude,
		Longitude:   data.Longitude,
		IsPrimary:   data.IsPrimary,
		IsActive:    data.IsActive,
		CreatedAt:   data.CreatedAt,
		UpdatedAt:   data.UpdatedAt,
	}
}

// fromAddressDomain converts a domain Address entity to a GORM AddressModel.
func fromAddressDomain(data *entity.Address) *model.AddressModel {
	if data == nil {
		return nil
	}

	addressModel := &model.AddressModel{
		ID:          data.ID,
		Label:       data.Label,
		FullAddress: data.FullAddress,
		Latitude:    data.Latitude,
		Longitude:   data.Longitude,
		IsPrimary:   data.IsPrimary,
		IsActive:    data.IsActive,
		CreatedAt:   data.CreatedAt,
		UpdatedAt:   data.UpdatedAt,
	}

	// Set the appropriate FK field based on owner type
	switch data.OwnerType {
	case entity.OwnerTypeUserProfile:
		addressModel.UserProfileID = &data.OwnerID
	case entity.OwnerTypeMerchantProfile:
		addressModel.MerchantProfileID = &data.OwnerID
	}

	return addressModel
}
