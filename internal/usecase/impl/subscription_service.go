package impl

import (
	"context"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
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
	if userID == merchantID {
		return nil, domainerrors.ErrSelfSubscriptionNotAllowed.WrapMessage("cannot subscribe to self")
	}

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
	}

	// Register device if provided
	if deviceInfo != nil {
		if err := s.registerDevice(ctx, userID, deviceInfo); err != nil {
			return nil, err
		}
	}

	updatedSubscription, err := s.subscriptionRepo.FindSubscriptionByID(ctx, sub.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to reload subscription after reactivation")
	}

	return updatedSubscription, nil
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

	createdSubscription, err := s.subscriptionRepo.FindSubscriptionByID(ctx, subscription.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to reload subscription after creation")
	}

	return createdSubscription, nil
}

// UnsubscribeFromMerchant deactivates a subscription (soft delete)
func (s *subscriptionService) UnsubscribeFromMerchant(ctx context.Context, userID, merchantID uuid.UUID) error {
	// Find subscription
	subscription, err := s.subscriptionRepo.FindSubscriptionByUserAndMerchant(ctx, userID, merchantID)
	if err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return domainerrors.ErrSubscriptionNotFound
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
		return nil, domainerrors.ErrInvalidQRCode
	}

	// Subscribe to merchant
	return s.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
}

// registerDevice is a helper function to register a device
func (s *subscriptionService) registerDevice(ctx context.Context, userID uuid.UUID, deviceInfo *usecase.DeviceInfo) error {
	_, err := upsertUserDevice(ctx, s.deviceRepo, userID, deviceInfo)
	if err != nil {
		return err
	}

	return nil
}
