package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// SubscriptionUsecase defines the interface for subscription management use cases
type SubscriptionUsecase interface {
	// SubscribeToMerchant creates or reactivates a subscription to a merchant
	SubscribeToMerchant(ctx context.Context, userID, merchantID uuid.UUID, deviceInfo *DeviceInfo) (*entity.UserMerchantSubscription, error)

	// UnsubscribeFromMerchant deactivates a subscription (soft delete)
	UnsubscribeFromMerchant(ctx context.Context, userID, merchantID uuid.UUID) error

	// GetUserSubscriptions retrieves all subscriptions for a user
	GetUserSubscriptions(ctx context.Context, userID uuid.UUID) ([]*entity.UserMerchantSubscription, error)

	// GetMerchantSubscribers retrieves all subscribers for a merchant
	GetMerchantSubscribers(ctx context.Context, merchantID uuid.UUID) ([]*entity.UserMerchantSubscription, error)

	// GenerateSubscriptionQR generates a QR code for merchant subscription
	GenerateSubscriptionQR(ctx context.Context, merchantID uuid.UUID) ([]byte, error)

	// ProcessQRSubscription processes a QR code subscription and optionally registers a device
	ProcessQRSubscription(ctx context.Context, userID uuid.UUID, qrData string, deviceInfo *DeviceInfo) (*entity.UserMerchantSubscription, error)
}
