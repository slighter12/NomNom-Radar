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

// userRepository implements the domain.UserRepository interface using GORM.
type userRepository struct {
	fx.In

	q *query.Query
}

// NewUserRepository is the constructor for userRepository.
// It initializes the repository with a database connection and the GORM Gen query builder.
// It returns the repository as a domain.UserRepository interface, adhering to dependency inversion.
func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepository{
		q: query.Use(db),
	}
}

// FindByID retrieves a single user by their unique ID, preloading their associated profiles and addresses.
func (repo *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	// Use the type-safe query builder 'Q' with proper preloading of nested associations
	userM, err := repo.q.UserModel.WithContext(ctx).
		Preload(repo.q.UserModel.UserProfile.Addresses).
		Preload(repo.q.UserModel.MerchantProfile.Addresses).
		Where(repo.q.UserModel.ID.Eq(id)).
		First() // First() returns a *postgres.UserModel

	if err != nil {
		// If the error is 'record not found', return a domain-specific error.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrUserNotFound
		}
		// Otherwise, return the original database error.
		return nil, errors.Wrap(err, "failed to find user by id")
	}

	// Map the persistence model back to a pure domain entity before returning.
	return toUserDomain(userM), nil
}

// FindByEmail retrieves a single user by their email address, preloading profiles and addresses.
func (repo *userRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	userM, err := repo.q.UserModel.WithContext(ctx).
		Preload(repo.q.UserModel.UserProfile.Addresses).
		Preload(repo.q.UserModel.MerchantProfile.Addresses).
		Where(repo.q.UserModel.Email.Eq(email)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrUserNotFound
		}

		return nil, errors.Wrap(err, "failed to find user by email")
	}

	return toUserDomain(userM), nil
}

// Create persists a new user entity, including its associated profiles, to the database.
// GORM's Create with associations will handle inserting into users, user_profiles,
// and/or merchant_profiles within a single transaction.
func (repo *userRepository) Create(ctx context.Context, user *entity.User) error {
	// Map the pure domain entity to a GORM persistence model.
	userM := fromUserDomain(user)

	// Execute the creation using the database connection.
	if err := repo.q.UserModel.WithContext(ctx).Create(userM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrUserAlreadyExists.WrapMessage("email already exists")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required user information")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid foreign key reference")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create user")
	}

	// Update the user entity with the generated ID and timestamps
	user.ID = userM.ID
	user.CreatedAt = userM.CreatedAt
	user.UpdatedAt = userM.UpdatedAt

	// Update profile IDs if they exist
	if user.UserProfile != nil && userM.UserProfile != nil {
		user.UserProfile.UserID = userM.UserProfile.UserID
		user.UserProfile.UpdatedAt = userM.UserProfile.UpdatedAt
	}
	if user.MerchantProfile != nil && userM.MerchantProfile != nil {
		user.MerchantProfile.UserID = userM.MerchantProfile.UserID
		user.MerchantProfile.UpdatedAt = userM.MerchantProfile.UpdatedAt
	}

	return nil
}

// Update modifies an existing user entity, including its associated profiles, in the database.
func (repo *userRepository) Update(ctx context.Context, user *entity.User) error {
	// Map the pure domain entity to a GORM persistence model.
	userM := fromUserDomain(user)

	// Execute the update using the database connection.
	// Use Session with FullSaveAssociations to update nested associations
	if err := repo.q.UserModel.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Save(userM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrUserAlreadyExists.WrapMessage("email already exists")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("missing required user information")
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserUpdateFailed.WrapMessage("invalid foreign key reference")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to update user")
	}

	// Update the user entity with the updated timestamps
	user.UpdatedAt = userM.UpdatedAt
	if user.UserProfile != nil && userM.UserProfile != nil {
		user.UserProfile.UpdatedAt = userM.UserProfile.UpdatedAt
	}
	if user.MerchantProfile != nil && userM.MerchantProfile != nil {
		user.MerchantProfile.UpdatedAt = userM.MerchantProfile.UpdatedAt
	}

	return nil
}

// --- Mapper Functions ---
// These helpers convert between domain entities and persistence models.

// toUserDomain converts a GORM UserModel to a domain User entity.
func toUserDomain(data *model.UserModel) *entity.User {
	if data == nil {
		return nil
	}

	return &entity.User{
		ID:              data.ID,
		Email:           data.Email,
		Name:            data.Name,
		UserProfile:     toUserProfileDomain(data.UserProfile),
		MerchantProfile: toMerchantProfileDomain(data.MerchantProfile),
		CreatedAt:       data.CreatedAt,
		UpdatedAt:       data.UpdatedAt,
	}
}

// fromUserDomain converts a domain User entity to a GORM UserModel for persistence.
func fromUserDomain(data *entity.User) *model.UserModel {
	if data == nil {
		return nil
	}

	return &model.UserModel{
		ID:              data.ID,
		Email:           data.Email,
		Name:            data.Name,
		UserProfile:     fromUserProfileDomain(data.UserProfile),
		MerchantProfile: fromMerchantProfileDomain(data.MerchantProfile),
	}
}

// toUserProfileDomain converts a GORM UserProfileModel to a domain UserProfile entity.
func toUserProfileDomain(data *model.UserProfileModel) *entity.UserProfile {
	if data == nil {
		return nil
	}

	addresses := make([]*entity.Address, 0, len(data.Addresses))
	for _, addr := range data.Addresses {
		// Determine owner ID and type from nullable FK fields
		var ownerID uuid.UUID
		var ownerType entity.OwnerType

		if addr.UserProfileID != nil {
			ownerID = *addr.UserProfileID
			ownerType = entity.OwnerTypeUserProfile
		} else if addr.MerchantProfileID != nil {
			ownerID = *addr.MerchantProfileID
			ownerType = entity.OwnerTypeMerchantProfile
		}

		addresses = append(addresses, &entity.Address{
			ID:          addr.ID,
			OwnerID:     ownerID,
			OwnerType:   ownerType,
			Label:       addr.Label,
			FullAddress: addr.FullAddress,
			Latitude:    addr.Latitude,
			Longitude:   addr.Longitude,
			IsPrimary:   addr.IsPrimary,
			CreatedAt:   addr.CreatedAt,
			UpdatedAt:   addr.UpdatedAt,
		})
	}

	return &entity.UserProfile{
		UserID:        data.UserID,
		Addresses:     addresses,
		LoyaltyPoints: data.LoyaltyPoints,
		UpdatedAt:     data.UpdatedAt,
	}
}

// fromUserProfileDomain converts a domain UserProfile entity to a GORM UserProfileModel.
func fromUserProfileDomain(data *entity.UserProfile) *model.UserProfileModel {
	if data == nil {
		return nil
	}

	addresses := make([]*model.AddressModel, 0, len(data.Addresses))
	for _, addr := range data.Addresses {
		addressModel := &model.AddressModel{
			ID:          addr.ID,
			Label:       addr.Label,
			FullAddress: addr.FullAddress,
			Latitude:    addr.Latitude,
			Longitude:   addr.Longitude,
			IsPrimary:   addr.IsPrimary,
			CreatedAt:   addr.CreatedAt,
			UpdatedAt:   addr.UpdatedAt,
		}

		// Set the appropriate FK field based on owner type
		switch addr.OwnerType {
		case entity.OwnerTypeUserProfile:
			addressModel.UserProfileID = &addr.OwnerID
		case entity.OwnerTypeMerchantProfile:
			addressModel.MerchantProfileID = &addr.OwnerID
		default:
			// Default to user profile if not specified
			userID := data.UserID
			addressModel.UserProfileID = &userID
		}

		addresses = append(addresses, addressModel)
	}

	return &model.UserProfileModel{
		UserID:        data.UserID,
		Addresses:     addresses,
		LoyaltyPoints: data.LoyaltyPoints,
		UpdatedAt:     data.UpdatedAt,
	}
}

// toMerchantProfileDomain converts a GORM MerchantProfileModel to a domain MerchantProfile entity.
func toMerchantProfileDomain(data *model.MerchantProfileModel) *entity.MerchantProfile {
	if data == nil {
		return nil
	}

	addresses := make([]*entity.Address, 0, len(data.Addresses))
	for _, addr := range data.Addresses {
		// Determine owner ID and type from nullable FK fields
		var ownerID uuid.UUID
		var ownerType entity.OwnerType

		if addr.UserProfileID != nil {
			ownerID = *addr.UserProfileID
			ownerType = entity.OwnerTypeUserProfile
		} else if addr.MerchantProfileID != nil {
			ownerID = *addr.MerchantProfileID
			ownerType = entity.OwnerTypeMerchantProfile
		}

		addresses = append(addresses, &entity.Address{
			ID:          addr.ID,
			OwnerID:     ownerID,
			OwnerType:   ownerType,
			Label:       addr.Label,
			FullAddress: addr.FullAddress,
			Latitude:    addr.Latitude,
			Longitude:   addr.Longitude,
			IsPrimary:   addr.IsPrimary,
			CreatedAt:   addr.CreatedAt,
			UpdatedAt:   addr.UpdatedAt,
		})
	}

	return &entity.MerchantProfile{
		UserID:           data.UserID,
		StoreName:        data.StoreName,
		StoreDescription: data.StoreDescription,
		BusinessLicense:  data.BusinessLicense,
		Addresses:        addresses,
		UpdatedAt:        data.UpdatedAt,
	}
}

// fromMerchantProfileDomain converts a domain MerchantProfile entity to a GORM MerchantProfileModel.
func fromMerchantProfileDomain(data *entity.MerchantProfile) *model.MerchantProfileModel {
	if data == nil {
		return nil
	}

	addresses := make([]*model.AddressModel, 0, len(data.Addresses))
	for _, addr := range data.Addresses {
		addressModel := &model.AddressModel{
			ID:          addr.ID,
			Label:       addr.Label,
			FullAddress: addr.FullAddress,
			Latitude:    addr.Latitude,
			Longitude:   addr.Longitude,
			IsPrimary:   addr.IsPrimary,
			CreatedAt:   addr.CreatedAt,
			UpdatedAt:   addr.UpdatedAt,
		}

		// Set the appropriate FK field based on owner type
		switch addr.OwnerType {
		case entity.OwnerTypeUserProfile:
			addressModel.UserProfileID = &addr.OwnerID
		case entity.OwnerTypeMerchantProfile:
			addressModel.MerchantProfileID = &addr.OwnerID
		default:
			// Default to merchant profile if not specified
			userID := data.UserID
			addressModel.MerchantProfileID = &userID
		}

		addresses = append(addresses, addressModel)
	}

	return &model.MerchantProfileModel{
		UserID:           data.UserID,
		StoreName:        data.StoreName,
		StoreDescription: data.StoreDescription,
		BusinessLicense:  data.BusinessLicense,
		Addresses:        addresses,
		UpdatedAt:        data.UpdatedAt,
	}
}
