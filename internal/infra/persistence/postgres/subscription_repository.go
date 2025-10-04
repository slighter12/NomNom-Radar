// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// subscriptionRepository implements the repository.SubscriptionRepository interface.
type subscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository is the constructor for subscriptionRepository.
func NewSubscriptionRepository(db *gorm.DB) repository.SubscriptionRepository {
	return &subscriptionRepository{
		db: db,
	}
}

// CreateSubscription persists a new subscription relationship.
func (repo *subscriptionRepository) CreateSubscription(ctx context.Context, subscription *entity.UserMerchantSubscription) error {
	subscriptionM := fromSubscriptionDomain(subscription)

	if err := repo.db.WithContext(ctx).Create(subscriptionM).Error; err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return repository.ErrDuplicateSubscription
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid user or merchant reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required subscription information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create subscription")
	}

	// Update the entity with generated values
	subscription.ID = subscriptionM.ID
	subscription.SubscribedAt = subscriptionM.SubscribedAt
	subscription.UpdatedAt = subscriptionM.UpdatedAt

	return nil
}

// FindSubscriptionByID retrieves a subscription by its unique ID.
func (repo *subscriptionRepository) FindSubscriptionByID(ctx context.Context, id uuid.UUID) (*entity.UserMerchantSubscription, error) {
	var subscriptionM model.UserMerchantSubscriptionModel

	if err := repo.db.WithContext(ctx).
		Where("id = ?", id).
		First(&subscriptionM).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrSubscriptionNotFound
		}

		return nil, errors.Wrap(err, "failed to find subscription by ID")
	}

	return toSubscriptionDomain(&subscriptionM), nil
}

// FindSubscriptionByUserAndMerchant retrieves a subscription by user and merchant IDs.
func (repo *subscriptionRepository) FindSubscriptionByUserAndMerchant(ctx context.Context, userID, merchantID uuid.UUID) (*entity.UserMerchantSubscription, error) {
	var subscriptionM model.UserMerchantSubscriptionModel

	if err := repo.db.WithContext(ctx).
		Where("user_id = ? AND merchant_id = ?", userID, merchantID).
		First(&subscriptionM).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrSubscriptionNotFound
		}

		return nil, errors.Wrap(err, "failed to find subscription by user and merchant")
	}

	return toSubscriptionDomain(&subscriptionM), nil
}

// FindSubscriptionsByUser retrieves all subscriptions for a specific user (excluding soft-deleted).
func (repo *subscriptionRepository) FindSubscriptionsByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	var subscriptionModels []*model.UserMerchantSubscriptionModel

	if err := repo.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("subscribed_at DESC").
		Find(&subscriptionModels).Error; err != nil {
		return nil, errors.Wrap(err, "failed to find subscriptions by user")
	}

	subscriptions := make([]*entity.UserMerchantSubscription, 0, len(subscriptionModels))
	for _, subscriptionM := range subscriptionModels {
		subscriptions = append(subscriptions, toSubscriptionDomain(subscriptionM))
	}

	return subscriptions, nil
}

// FindSubscriptionsByMerchant retrieves all subscriptions for a specific merchant (excluding soft-deleted).
func (repo *subscriptionRepository) FindSubscriptionsByMerchant(ctx context.Context, merchantID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	var subscriptionModels []*model.UserMerchantSubscriptionModel

	if err := repo.db.WithContext(ctx).
		Where("merchant_id = ?", merchantID).
		Order("subscribed_at DESC").
		Find(&subscriptionModels).Error; err != nil {
		return nil, errors.Wrap(err, "failed to find subscriptions by merchant")
	}

	subscriptions := make([]*entity.UserMerchantSubscription, 0, len(subscriptionModels))
	for _, subscriptionM := range subscriptionModels {
		subscriptions = append(subscriptions, toSubscriptionDomain(subscriptionM))
	}

	return subscriptions, nil
}

// UpdateSubscriptionStatus updates the active status of a subscription.
func (repo *subscriptionRepository) UpdateSubscriptionStatus(ctx context.Context, id uuid.UUID, isActive bool) error {
	result := repo.db.WithContext(ctx).
		Model(&model.UserMerchantSubscriptionModel{}).
		Where("id = ?", id).
		Update("is_active", isActive)

	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to update subscription status")
	}

	if result.RowsAffected == 0 {
		return repository.ErrSubscriptionNotFound
	}

	return nil
}

// UpdateNotificationRadius updates the notification radius for a subscription.
func (repo *subscriptionRepository) UpdateNotificationRadius(ctx context.Context, id uuid.UUID, radius float64) error {
	result := repo.db.WithContext(ctx).
		Model(&model.UserMerchantSubscriptionModel{}).
		Where("id = ?", id).
		Update("notification_radius", radius)

	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to update notification radius")
	}

	if result.RowsAffected == 0 {
		return repository.ErrSubscriptionNotFound
	}

	return nil
}

// DeleteSubscription removes a subscription by its ID (soft delete).
func (repo *subscriptionRepository) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	result := repo.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.UserMerchantSubscriptionModel{})

	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to delete subscription")
	}

	if result.RowsAffected == 0 {
		return repository.ErrSubscriptionNotFound
	}

	return nil
}

// FindSubscribersWithinRadius performs a PostGIS geographic query to find all active subscriptions
// where the user has at least one active address within the notification radius of the merchant's location.
func (repo *subscriptionRepository) FindSubscribersWithinRadius(ctx context.Context, merchantID uuid.UUID, merchantLat, merchantLon float64) ([]*entity.UserMerchantSubscription, error) {
	var subscriptionModels []*model.UserMerchantSubscriptionModel

	// Use PostGIS ST_DWithin for efficient geographic queries
	// This query finds distinct subscriptions where the user has at least one active address within range
	query := `
		SELECT DISTINCT s.*
		FROM user_merchant_subscriptions s
		WHERE s.merchant_id = ?
		  AND s.is_active = true
		  AND s.deleted_at IS NULL
		  AND EXISTS (
		    SELECT 1
		    FROM addresses a
		    WHERE a.owner_id = s.user_id
		      AND a.owner_type = 'user_profile'
		      AND a.is_active = true
		      AND a.deleted_at IS NULL
		      AND ST_DWithin(
		        a.location,
		        ST_SetSRID(ST_MakePoint(?, ?), 4326),
		        s.notification_radius
		      )
		  )
		ORDER BY s.subscribed_at DESC
	`

	if err := repo.db.WithContext(ctx).
		Raw(query, merchantID, merchantLon, merchantLat).
		Scan(&subscriptionModels).Error; err != nil {
		return nil, errors.Wrap(err, "failed to find subscribers within radius")
	}

	subscriptions := make([]*entity.UserMerchantSubscription, 0, len(subscriptionModels))
	for _, subscriptionM := range subscriptionModels {
		subscriptions = append(subscriptions, toSubscriptionDomain(subscriptionM))
	}

	return subscriptions, nil
}

// FindDevicesForUsers retrieves all active devices for a list of user IDs.
func (repo *subscriptionRepository) FindDevicesForUsers(ctx context.Context, userIDs []uuid.UUID) ([]*entity.UserDevice, error) {
	if len(userIDs) == 0 {
		return []*entity.UserDevice{}, nil
	}

	var deviceModels []*model.UserDeviceModel

	if err := repo.db.WithContext(ctx).
		Where("user_id IN ? AND is_active = ?", userIDs, true).
		Order("created_at DESC").
		Find(&deviceModels).Error; err != nil {
		return nil, errors.Wrap(err, "failed to find devices for users")
	}

	devices := make([]*entity.UserDevice, 0, len(deviceModels))
	for _, deviceM := range deviceModels {
		devices = append(devices, toDeviceDomain(deviceM))
	}

	return devices, nil
}

// --- Mapper Functions ---

// toSubscriptionDomain converts a GORM UserMerchantSubscriptionModel to a domain UserMerchantSubscription entity.
func toSubscriptionDomain(data *model.UserMerchantSubscriptionModel) *entity.UserMerchantSubscription {
	if data == nil {
		return nil
	}

	return &entity.UserMerchantSubscription{
		ID:                 data.ID,
		UserID:             data.UserID,
		MerchantID:         data.MerchantID,
		IsActive:           data.IsActive,
		NotificationRadius: data.NotificationRadius,
		SubscribedAt:       data.SubscribedAt,
		UpdatedAt:          data.UpdatedAt,
	}
}

// fromSubscriptionDomain converts a domain UserMerchantSubscription entity to a GORM UserMerchantSubscriptionModel.
func fromSubscriptionDomain(data *entity.UserMerchantSubscription) *model.UserMerchantSubscriptionModel {
	if data == nil {
		return nil
	}

	return &model.UserMerchantSubscriptionModel{
		ID:                 data.ID,
		UserID:             data.UserID,
		MerchantID:         data.MerchantID,
		IsActive:           data.IsActive,
		NotificationRadius: data.NotificationRadius,
		SubscribedAt:       data.SubscribedAt,
		UpdatedAt:          data.UpdatedAt,
	}
}
