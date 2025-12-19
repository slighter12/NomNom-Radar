// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"
	"database/sql/driver"

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

// subscriptionRepository implements the repository.SubscriptionRepository interface.
type subscriptionRepository struct {
	fx.In

	q *query.Query
}

// NewSubscriptionRepository is the constructor for subscriptionRepository.
func NewSubscriptionRepository(db *gorm.DB) repository.SubscriptionRepository {
	return &subscriptionRepository{
		q: query.Use(db),
	}
}

// CreateSubscription persists a new subscription relationship.
func (repo *subscriptionRepository) CreateSubscription(ctx context.Context, subscription *entity.UserMerchantSubscription) error {
	subscriptionM := fromSubscriptionDomain(subscription)

	if err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).Create(subscriptionM); err != nil {
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
	subscriptionM, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(repo.q.UserMerchantSubscriptionModel.ID.Eq(id)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrSubscriptionNotFound
		}

		return nil, errors.Wrap(err, "failed to find subscription by ID")
	}

	return toSubscriptionDomain(subscriptionM), nil
}

// FindSubscriptionByUserAndMerchant retrieves a subscription by user and merchant IDs.
func (repo *subscriptionRepository) FindSubscriptionByUserAndMerchant(ctx context.Context, userID, merchantID uuid.UUID) (*entity.UserMerchantSubscription, error) {
	subscriptionM, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(
			repo.q.UserMerchantSubscriptionModel.UserID.Eq(userID),
			repo.q.UserMerchantSubscriptionModel.MerchantID.Eq(merchantID),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrSubscriptionNotFound
		}

		return nil, errors.Wrap(err, "failed to find subscription by user and merchant")
	}

	return toSubscriptionDomain(subscriptionM), nil
}

// FindSubscriptionsByUser retrieves all subscriptions for a specific user (excluding soft-deleted).
func (repo *subscriptionRepository) FindSubscriptionsByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	subscriptionModels, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(repo.q.UserMerchantSubscriptionModel.UserID.Eq(userID)).
		Order(repo.q.UserMerchantSubscriptionModel.SubscribedAt.Desc()).
		Find()

	if err != nil {
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
	subscriptionModels, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(repo.q.UserMerchantSubscriptionModel.MerchantID.Eq(merchantID)).
		Order(repo.q.UserMerchantSubscriptionModel.SubscribedAt.Desc()).
		Find()

	if err != nil {
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
	result, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(repo.q.UserMerchantSubscriptionModel.ID.Eq(id)).
		Update(repo.q.UserMerchantSubscriptionModel.IsActive, isActive)

	if err != nil {
		return errors.Wrap(err, "failed to update subscription status")
	}

	if result.RowsAffected == 0 {
		return repository.ErrSubscriptionNotFound
	}

	return nil
}

// UpdateNotificationRadius updates the notification radius for a subscription.
func (repo *subscriptionRepository) UpdateNotificationRadius(ctx context.Context, id uuid.UUID, radius float64) error {
	result, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(repo.q.UserMerchantSubscriptionModel.ID.Eq(id)).
		Update(repo.q.UserMerchantSubscriptionModel.NotificationRadius, radius)

	if err != nil {
		return errors.Wrap(err, "failed to update notification radius")
	}

	if result.RowsAffected == 0 {
		return repository.ErrSubscriptionNotFound
	}

	return nil
}

// DeleteSubscription removes a subscription by its ID (soft delete).
func (repo *subscriptionRepository) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.UserMerchantSubscriptionModel.WithContext(ctx).
		Where(repo.q.UserMerchantSubscriptionModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return errors.Wrap(err, "failed to delete subscription")
	}

	if result.RowsAffected == 0 {
		return repository.ErrSubscriptionNotFound
	}

	return nil
}

// FindSubscribersWithinRadius performs a PostGIS geographic query to find all active subscriptions
// where the user has at least one active address within the notification radius of the merchant's location.
func (repo *subscriptionRepository) FindSubscribersWithinRadius(ctx context.Context, merchantID uuid.UUID, merchantLat, merchantLon float64) ([]*entity.UserMerchantSubscription, error) {
	s := repo.q.UserMerchantSubscriptionModel
	a := repo.q.AddressModel

	// Build subquery for EXISTS check using GORM fluent API
	subQuery := a.WithContext(ctx).Select(a.ID).Where(
		a.UserProfileID.EqCol(s.UserID),
		a.IsActive.Is(true),
		a.DeletedAt.IsNull(),
	).UnderlyingDB().Where("ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326), notification_radius)", merchantLon, merchantLat)

	var subscriptionModels []*model.UserMerchantSubscriptionModel
	err := s.WithContext(ctx).
		Where(
			s.MerchantID.Eq(merchantID),
			s.IsActive.Is(true),
			s.DeletedAt.IsNull(),
		).UnderlyingDB().
		Where("EXISTS (?)", subQuery).
		Order("subscribed_at DESC").
		Find(&subscriptionModels).Error

	if err != nil {
		return nil, errors.Wrap(err, "failed to find subscribers within radius")
	}

	subscriptions := make([]*entity.UserMerchantSubscription, 0, len(subscriptionModels))
	for _, subscriptionM := range subscriptionModels {
		subscriptions = append(subscriptions, toSubscriptionDomain(subscriptionM))
	}

	return subscriptions, nil
}

// FindSubscriberAddressesWithinRadius performs a PostGIS geographic query to find all active addresses
// within the notification radius of the merchant's location for active subscriptions.
func (repo *subscriptionRepository) FindSubscriberAddressesWithinRadius(ctx context.Context, merchantID uuid.UUID, merchantLat, merchantLon float64) ([]*entity.Address, error) {
	a := repo.q.AddressModel
	s := repo.q.UserMerchantSubscriptionModel

	// Construct complex query using fluent API for structure and UnderlyingDB for PostGIS specifics
	var addressModels []*model.AddressModel
	err := a.WithContext(ctx).
		Distinct().
		Join(s, s.UserID.EqCol(a.UserProfileID)).
		Where(
			a.UserProfileID.IsNotNull(),
			a.IsActive.Is(true),
			a.DeletedAt.IsNull(),
			s.MerchantID.Eq(merchantID),
			s.IsActive.Is(true),
			s.DeletedAt.IsNull(),
		).UnderlyingDB().
		Where("ST_DWithin(a.location, ST_SetSRID(ST_MakePoint(?, ?), 4326), s.notification_radius)", merchantLon, merchantLat).
		Order(gorm.Expr("ST_Distance(a.location, ST_SetSRID(ST_MakePoint(?, ?), 4326))", merchantLon, merchantLat)).
		Find(&addressModels).Error

	if err != nil {
		return nil, errors.Wrap(err, "failed to find subscriber addresses within radius")
	}

	addresses := make([]*entity.Address, 0, len(addressModels))
	for _, addressM := range addressModels {
		addresses = append(addresses, toAddressDomain(addressM))
	}

	return addresses, nil
}

// FindDevicesForUsers retrieves all active devices for a list of user IDs.
func (repo *subscriptionRepository) FindDevicesForUsers(ctx context.Context, userIDs []uuid.UUID) ([]*entity.UserDevice, error) {
	if len(userIDs) == 0 {
		return []*entity.UserDevice{}, nil
	}

	// uuid.UUID implements driver.Valuer, so we convert slice for type safety with gen.Field.In
	ids := make([]driver.Valuer, len(userIDs))
	for i, id := range userIDs {
		ids[i] = id
	}

	deviceModels, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(
			repo.q.UserDeviceModel.UserID.In(ids...),
			repo.q.UserDeviceModel.IsActive.Is(true),
		).
		Order(repo.q.UserDeviceModel.CreatedAt.Desc()).
		Find()

	if err != nil {
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
