package impl

import (
	"context"
	"errors"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
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

// NewSubscriptionService creates a new subscription service instance
func NewSubscriptionService(
	subscriptionRepo repository.SubscriptionRepository,
	deviceRepo repository.DeviceRepository,
	qrcodeService service.QRCodeService,
	cfg *config.Config,
) usecase.SubscriptionUsecase {
	return &subscriptionService{
		subscriptionRepo: subscriptionRepo,
		deviceRepo:       deviceRepo,
		qrcodeService:    qrcodeService,
		config:           cfg,
	}
}

// SubscribeToMerchant creates or reactivates a subscription to a merchant
func (s *subscriptionService) SubscribeToMerchant(ctx context.Context, userID, merchantID uuid.UUID, deviceInfo *usecase.DeviceInfo) (*entity.UserMerchantSubscription, error) {
	// Check if subscription already exists
	existingSub, err := s.subscriptionRepo.FindSubscriptionByUserAndMerchant(ctx, userID, merchantID)
	if err != nil && !errors.Is(err, repository.ErrSubscriptionNotFound) {
		return nil, err
	}

	// If subscription exists, reactivate it
	if existingSub != nil {
		if !existingSub.IsActive {
			if err := s.subscriptionRepo.UpdateSubscriptionStatus(ctx, existingSub.ID, true); err != nil {
				return nil, err
			}
			existingSub.IsActive = true
			existingSub.UpdatedAt = time.Now()
		}

		// Register device if provided
		if deviceInfo != nil {
			if err := s.registerDevice(ctx, userID, deviceInfo); err != nil {
				return nil, err
			}
		}

		return existingSub, nil
	}

	// Create new subscription
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
		return nil, err
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
		return err
	}

	return s.subscriptionRepo.DeleteSubscription(ctx, subscription.ID)
}

// GetUserSubscriptions retrieves all subscriptions for a user
func (s *subscriptionService) GetUserSubscriptions(ctx context.Context, userID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	return s.subscriptionRepo.FindSubscriptionsByUser(ctx, userID)
}

// GetMerchantSubscribers retrieves all subscribers for a merchant
func (s *subscriptionService) GetMerchantSubscribers(ctx context.Context, merchantID uuid.UUID) ([]*entity.UserMerchantSubscription, error) {
	return s.subscriptionRepo.FindSubscriptionsByMerchant(ctx, merchantID)
}

// UpdateNotificationRadius updates the notification radius for a subscription
func (s *subscriptionService) UpdateNotificationRadius(ctx context.Context, userID, merchantID uuid.UUID, radius float64) error {
	// Validate radius
	maxRadius := s.config.LocationNotification.MaxRadius
	if radius <= 0 || radius > maxRadius {
		return ErrInvalidRadius
	}

	// Find subscription
	subscription, err := s.subscriptionRepo.FindSubscriptionByUserAndMerchant(ctx, userID, merchantID)
	if err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return ErrSubscriptionNotFound
		}
		return err
	}

	return s.subscriptionRepo.UpdateNotificationRadius(ctx, subscription.ID, radius)
}

// GenerateSubscriptionQR generates a QR code for merchant subscription
func (s *subscriptionService) GenerateSubscriptionQR(ctx context.Context, merchantID uuid.UUID) ([]byte, error) {
	return s.qrcodeService.GenerateSubscriptionQR(merchantID)
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
		return err
	}

	// Look for existing device with same device_id
	for _, device := range devices {
		if device.DeviceID == deviceInfo.DeviceID {
			// Update FCM token for existing device
			return s.deviceRepo.UpdateFCMToken(ctx, device.ID, deviceInfo.FCMToken)
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

	return s.deviceRepo.CreateDevice(ctx, device)
}
