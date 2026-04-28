package repository

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
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

	// FindSubscriberAddressesWithinRadius performs a PostGIS geographic query to find all active addresses
	// within the notification radius of the merchant's location for active subscriptions.
	// Returns addresses bundled with their subscription notification radius to avoid N+1 lookups.
	FindSubscriberAddressesWithinRadius(ctx context.Context, merchantID uuid.UUID, merchantLat, merchantLon float64) ([]*entity.SubscriberAddress, error)

	// FindDevicesForUsers retrieves healthy devices for a list of user IDs.
	// Healthy means active, non-deleted, and token refreshed within the health window.
	FindDevicesForUsers(ctx context.Context, userIDs []uuid.UUID, healthyWindowDays int) ([]*entity.UserDevice, error)

	// FindSubscriberAddressesByUserIDs retrieves addresses for specific user IDs who subscribe to a merchant.
	// Returns addresses bundled with their subscription notification radius.
	FindSubscriberAddressesByUserIDs(ctx context.Context, merchantID uuid.UUID, userIDs []uuid.UUID) ([]*entity.SubscriberAddress, error)
}
