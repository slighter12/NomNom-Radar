// Package repository defines the interfaces for the persistence layer.
package repository

import (
	"context"
	"errors"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// Domain-specific errors for subscription persistence.
var (
	// ErrSubscriptionNotFound is returned when a subscription is not found.
	ErrSubscriptionNotFound = errors.New("subscription not found")
	// ErrDuplicateSubscription is returned when trying to create a subscription that already exists.
	ErrDuplicateSubscription = errors.New("subscription already exists")
)

// SubscriptionRepository defines the interface for subscription-related database operations.
type SubscriptionRepository interface {
	// CreateSubscription persists a new subscription relationship.
	CreateSubscription(ctx context.Context, subscription *entity.UserMerchantSubscription) error

	// FindSubscriptionByID retrieves a subscription by its unique ID.
	FindSubscriptionByID(ctx context.Context, id uuid.UUID) (*entity.UserMerchantSubscription, error)

	// FindSubscriptionByUserAndMerchant retrieves a subscription by user and merchant IDs.
	FindSubscriptionByUserAndMerchant(ctx context.Context, userID, merchantID uuid.UUID) (*entity.UserMerchantSubscription, error)

	// FindSubscriptionsByUser retrieves all subscriptions for a specific user.
	FindSubscriptionsByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserMerchantSubscription, error)

	// FindSubscriptionsByMerchant retrieves all subscriptions for a specific merchant.
	FindSubscriptionsByMerchant(ctx context.Context, merchantID uuid.UUID) ([]*entity.UserMerchantSubscription, error)

	// UpdateSubscriptionStatus updates the active status of a subscription.
	UpdateSubscriptionStatus(ctx context.Context, id uuid.UUID, isActive bool) error

	// UpdateNotificationRadius updates the notification radius for a subscription.
	UpdateNotificationRadius(ctx context.Context, id uuid.UUID, radius float64) error

	// DeleteSubscription removes a subscription by its ID (soft delete).
	DeleteSubscription(ctx context.Context, id uuid.UUID) error

	// FindSubscribersWithinRadius performs a PostGIS geographic query to find all active subscriptions
	// where the user has at least one active address within the notification radius of the merchant's location.
	// Returns distinct user subscriptions to avoid duplicates when a user has multiple addresses in range.
	FindSubscribersWithinRadius(ctx context.Context, merchantID uuid.UUID, merchantLat, merchantLon float64) ([]*entity.UserMerchantSubscription, error)

	// FindDevicesForUsers retrieves all active devices for a list of user IDs.
	// Used for batch fetching devices for notification sending.
	FindDevicesForUsers(ctx context.Context, userIDs []uuid.UUID) ([]*entity.UserDevice, error)
}
