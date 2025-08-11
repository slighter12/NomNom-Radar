// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
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

// FindByID retrieves a single user by their unique ID, preloading their associated profiles.
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	// Use the type-safe query builder 'Q'
	userM, err := r.q.UserModel.WithContext(ctx).
		Preload(r.q.UserModel.UserProfile).
		Preload(r.q.UserModel.MerchantProfile).
		Where(r.q.UserModel.ID.Eq(id)).
		First() // First() returns a *postgres.UserModel

	if err != nil {
		// If the error is 'record not found', return a domain-specific error.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrUserNotFound
		}
		// Otherwise, return the original database error.
		return nil, err
	}

	// Map the persistence model back to a pure domain entity before returning.
	return toUserDomain(userM), nil
}

// FindByEmail retrieves a single user by their email address, preloading profiles.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	userM, err := r.q.UserModel.WithContext(ctx).
		Preload(r.q.UserModel.UserProfile).
		Preload(r.q.UserModel.MerchantProfile).
		Where(r.q.UserModel.Email.Eq(email)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrUserNotFound
		}
		return nil, err
	}

	return toUserDomain(userM), nil
}

// Create persists a new user entity, including its associated profiles, to the database.
// GORM's Create with associations will handle inserting into users, user_profiles,
// and/or merchant_profiles within a single transaction.
func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	// Map the pure domain entity to a GORM persistence model.
	userM := fromUserDomain(user)

	// Execute the creation using the database connection.
	return r.q.UserModel.WithContext(ctx).Create(userM)
}

// --- Mapper Functions ---
// These helpers convert between domain entities and persistence models.

// toUserDomain converts a GORM UserModel to a domain User entity.
func toUserDomain(m *model.UserModel) *entity.User {
	if m == nil {
		return nil
	}
	return &entity.User{
		ID:              m.ID,
		Email:           m.Email,
		Name:            m.Name,
		UserProfile:     toUserProfileDomain(m.UserProfile),
		MerchantProfile: toMerchantProfileDomain(m.MerchantProfile),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// fromUserDomain converts a domain User entity to a GORM UserModel for persistence.
func fromUserDomain(e *entity.User) *model.UserModel {
	if e == nil {
		return nil
	}
	return &model.UserModel{
		ID:              e.ID,
		Email:           e.Email,
		Name:            e.Name,
		UserProfile:     fromUserProfileDomain(e.UserProfile),
		MerchantProfile: fromMerchantProfileDomain(e.MerchantProfile),
	}
}

// toUserProfileDomain converts a GORM UserProfileModel to a domain UserProfile entity.
func toUserProfileDomain(m *model.UserProfileModel) *entity.UserProfile {
	if m == nil {
		return nil
	}
	return &entity.UserProfile{
		UserID:                 m.UserID,
		DefaultShippingAddress: m.DefaultShippingAddress,
		LoyaltyPoints:          m.LoyaltyPoints,
		UpdatedAt:              m.UpdatedAt,
	}
}

// fromUserProfileDomain converts a domain UserProfile entity to a GORM UserProfileModel.
func fromUserProfileDomain(e *entity.UserProfile) *model.UserProfileModel {
	if e == nil {
		return nil
	}
	// The UserID is set by the association when creating the parent userModel.
	return &model.UserProfileModel{
		UserID:                 e.UserID,
		DefaultShippingAddress: e.DefaultShippingAddress,
		LoyaltyPoints:          e.LoyaltyPoints,
	}
}

// toMerchantProfileDomain converts a GORM MerchantProfileModel to a domain MerchantProfile entity.
func toMerchantProfileDomain(m *model.MerchantProfileModel) *entity.MerchantProfile {
	if m == nil {
		return nil
	}
	return &entity.MerchantProfile{
		UserID:           m.UserID,
		StoreName:        m.StoreName,
		StoreDescription: m.StoreDescription,
		BusinessLicense:  m.BusinessLicense,
		StoreAddress:     m.StoreAddress,
		UpdatedAt:        m.UpdatedAt,
	}
}

// fromMerchantProfileDomain converts a domain MerchantProfile entity to a GORM MerchantProfileModel.
func fromMerchantProfileDomain(e *entity.MerchantProfile) *model.MerchantProfileModel {
	if e == nil {
		return nil
	}
	// The UserID is set by the association when creating the parent userModel.
	return &model.MerchantProfileModel{
		UserID:           e.UserID,
		StoreName:        e.StoreName,
		StoreDescription: e.StoreDescription,
		BusinessLicense:  e.BusinessLicense,
		StoreAddress:     e.StoreAddress,
	}
}
