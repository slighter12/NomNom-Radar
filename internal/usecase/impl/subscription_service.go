package impl

import (
	"context"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

var (
	// ErrSubscriptionNotFound is returned when a subscription is not found
	ErrSubscriptionNotFound = errors.New("subscription not found")
	// ErrInvalidRadius is returned when the notification radius is invalid
	ErrInvalidRadius = errors.New("invalid notification radius")
	// ErrInvalidQRCode is returned when the QR code is invalid
	ErrInvalidQRCode = errors.New("invalid QR code")
)

type subscriptionService struct {
	subscriptionRepo repository.SubscriptionRepository
	deviceRepo       repository.DeviceRepository
	qrcodeService    service.QRCodeService
	config           *config.Config
}

// SubscriptionServiceParams holds dependencies for SubscriptionService, injected by Fx.
type SubscriptionServiceParams struct {
	fx.In

	SubscriptionRepo repository.SubscriptionRepository
	DeviceRepo       repository.DeviceRepository
	QRCodeService    service.QRCodeService
	Config           *config.Config
}

// NewSubscriptionService creates a new subscription service instance
func NewSubscriptionService(params SubscriptionServiceParams) usecase.SubscriptionUsecase {
	return &subscriptionService{
		subscriptionRepo: params.SubscriptionRepo,
		deviceRepo:       params.DeviceRepo,
		qrcodeService:    params.QRCodeService,
		config:           params.Config,
	}
}

// SubscribeToMerchant creates or reactivates a subscription to a merchant
func (s *subscriptionService) SubscribeToMerchant(ctx context.Context, userID, merchantID uuid.UUID, deviceInfo *usecase.DeviceInfo) (*entity.UserMerchantSubscription, error) {
	// Check if subscription already exists
	existingSub, err := s.subscriptionRepo.FindSubscriptionByUserAndMerchant(ctx, userID, merchantID)
	if err != nil && !errors.Is(err, repository.ErrSubscriptionNotFound) {
		return nil, errors.Wrap(err, "failed to find subscription by user and merchant")
	}

	// If subscription exists, reactivate it
	if existingSub != nil {
		return s.reactivateSubscription(ctx, userID, existingSub, deviceInfo)
	}

	// Create new subscription
	return s.createNewSubscription(ctx, userID, merchantID, deviceInfo)
}

// reactivateSubscription reactivates an existing subscription
func (s *subscriptionService) reactivateSubscription(ctx context.Context, userID uuid.UUID, sub *entity.UserMerchantSubscription, deviceInfo *usecase.DeviceInfo) (*entity.UserMerchantSubscription, error) {
	if !sub.IsActive {
		if err := s.subscriptionRepo.UpdateSubscriptionStatus(ctx, sub.ID, true); err != nil {
			return nil, errors.Wrap(err, "failed to update subscription status")
		}
		sub.IsActive = true
		sub.UpdatedAt = time.Now()
	}

	// Register device if provided
	if deviceInfo != nil {
		if err := s.registerDevice(ctx, userID, deviceInfo); err != nil {
			return nil, err
		}
	}

	return sub, nil
}

// createNewSubscription creates a new subscription
func (s *subscriptionService) createNewSubscription(ctx context.Context, userID, merchantID uuid.UUID, deviceInfo *usecase.DeviceInfo) (*entity.UserMerchantSubscription, error) {
	subscription := &entity.UserMerchantSubscription{
		ID:                 uuid.New(),
		UserID:             userID,
		MerchantID:         merchantID,
		IsActive:           true,
		NotificationRadius: s.config.LocationNotification.DefaultRadius,
		SubscribedAt:       time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.subscriptionRepo.CreateSubscription(ctx, subscription); err != nil {
		return nil, errors.Wrap(err, "failed to create subscription")
	}

	// Register device if provided
	if deviceInfo != nil {
		if err := s.registerDevice(ctx, userID, deviceInfo); err != nil {
			return nil, err
		}
	}

	return subscription, nil
}

// UnsubscribeFromMerchant deactivates a subscription (soft delete)
func (s *subscriptionService) UnsubscribeFromMerchant(ctx context.Context, userID, merchantID uuid.UUID) error {
	// Find subscription
	subscription, err := s.subscriptionRepo.FindSubscriptionByUserAndMerchant(ctx, userID, merchantID)
	if err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return ErrSubscriptionNotFound
		}

		return errors.Wrap(err, "failed to find subscription by user and merchant")
	}

	if err := s.subscriptionRepo.DeleteSubscription(ctx, subscription.ID); err != nil {
		return errors.Wrap(err, "failed to delete subscription")
	}

	return nil
}

// GetUserSubscriptions retrieves all subscriptions for a user
func (s *subscriptionService) GetUserSubscriptions(ctx context.Context, userID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	subscriptions, err := s.subscriptionRepo.FindSubscriptionsByUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find subscriptions by user")
	}

	return subscriptions, nil
}

// GetMerchantSubscribers retrieves all subscribers for a merchant
func (s *subscriptionService) GetMerchantSubscribers(ctx context.Context, merchantID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	subscriptions, err := s.subscriptionRepo.FindSubscriptionsByMerchant(ctx, merchantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find subscriptions by merchant")
	}

	return subscriptions, nil
}

// GenerateSubscriptionQR generates a QR code for merchant subscription
func (s *subscriptionService) GenerateSubscriptionQR(ctx context.Context, merchantID uuid.UUID) ([]byte, error) {
	qrCode, err := s.qrcodeService.GenerateSubscriptionQR(merchantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate subscription QR")
	}

	return qrCode, nil
}

// ProcessQRSubscription processes a QR code subscription and optionally registers a device
func (s *subscriptionService) ProcessQRSubscription(ctx context.Context, userID uuid.UUID, qrData string, deviceInfo *usecase.DeviceInfo) (*entity.UserMerchantSubscription, error) {
	// Parse QR code to get merchant ID
	merchantID, err := s.qrcodeService.ParseSubscriptionQR(qrData)
	if err != nil {
		return nil, ErrInvalidQRCode
	}

	// Subscribe to merchant
	return s.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
}

// registerDevice is a helper function to register a device
func (s *subscriptionService) registerDevice(ctx context.Context, userID uuid.UUID, deviceInfo *usecase.DeviceInfo) error {
	// Check if device already exists for this user
	devices, err := s.deviceRepo.FindDevicesByUser(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "failed to find devices by user")
	}

	// Look for existing device with same device_id
	for _, device := range devices {
		if device.DeviceID == deviceInfo.DeviceID {
			// Update FCM token for existing device
			if err := s.deviceRepo.UpdateFCMToken(ctx, device.ID, deviceInfo.FCMToken); err != nil {
				return errors.Wrap(err, "failed to update FCM token")
			}

			return nil
		}
	}

	// Create new device
	device := &entity.UserDevice{
		ID:        uuid.New(),
		UserID:    userID,
		FCMToken:  deviceInfo.FCMToken,
		DeviceID:  deviceInfo.DeviceID,
		Platform:  deviceInfo.Platform,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.deviceRepo.CreateDevice(ctx, device); err != nil {
		return errors.Wrap(err, "failed to create device")
	}

	return nil
}
