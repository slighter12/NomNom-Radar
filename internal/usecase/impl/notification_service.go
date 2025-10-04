package impl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
)

var (
	// ErrInvalidNotificationData is returned when neither addressID nor locationData is provided
	ErrInvalidNotificationData = errors.New("either addressID or locationData must be provided")
)

const (
	// Firebase batch size limit
	firebaseBatchSize = 500
)

type notificationService struct {
	notificationRepo repository.NotificationRepository
	subscriptionRepo repository.SubscriptionRepository
	deviceRepo       repository.DeviceRepository
	addressRepo      repository.AddressRepository
	notificationSvc  service.NotificationService
}

// NewNotificationService creates a new notification service instance
func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	subscriptionRepo repository.SubscriptionRepository,
	deviceRepo repository.DeviceRepository,
	addressRepo repository.AddressRepository,
	notificationSvc service.NotificationService,
) usecase.NotificationUsecase {
	return &notificationService{
		notificationRepo: notificationRepo,
		subscriptionRepo: subscriptionRepo,
		deviceRepo:       deviceRepo,
		addressRepo:      addressRepo,
		notificationSvc:  notificationSvc,
	}
}

// PublishLocationNotification publishes a location notification to nearby subscribers
func (s *notificationService) PublishLocationNotification(
	ctx context.Context,
	merchantID uuid.UUID,
	addressID *uuid.UUID,
	locationData *usecase.LocationData,
	hintMessage string,
) (*entity.MerchantLocationNotification, error) {
	// Validate input
	if addressID == nil && locationData == nil {
		return nil, ErrInvalidNotificationData
	}

	var (
		locationName string
		fullAddress  string
		latitude     float64
		longitude    float64
	)

	// Get location information
	if addressID != nil {
		// Fetch address from repository
		address, err := s.addressRepo.FindAddressByID(ctx, *addressID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch address: %w", err)
		}

		// Verify ownership
		if address.OwnerID != merchantID || address.OwnerType != entity.OwnerTypeMerchantProfile {
			return nil, errors.New("unauthorized: address does not belong to merchant")
		}

		locationName = address.Label
		fullAddress = address.FullAddress
		latitude = address.Latitude
		longitude = address.Longitude
	} else {
		// Use provided location data
		locationName = locationData.LocationName
		fullAddress = locationData.FullAddress
		latitude = locationData.Latitude
		longitude = locationData.Longitude
	}

	// Create notification record
	notification := &entity.MerchantLocationNotification{
		ID:           uuid.New(),
		MerchantID:   merchantID,
		AddressID:    addressID,
		LocationName: locationName,
		FullAddress:  fullAddress,
		Latitude:     latitude,
		Longitude:    longitude,
		HintMessage:  hintMessage,
		TotalSent:    0,
		TotalFailed:  0,
		PublishedAt:  time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.notificationRepo.CreateNotification(ctx, notification); err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	// Find subscribers within radius using PostGIS
	subscriptions, err := s.subscriptionRepo.FindSubscribersWithinRadius(ctx, merchantID, latitude, longitude)
	if err != nil {
		return nil, fmt.Errorf("failed to find subscribers: %w", err)
	}

	// If no subscribers, return early
	if len(subscriptions) == 0 {
		return notification, nil
	}

	// Collect user IDs
	userIDs := make([]uuid.UUID, 0, len(subscriptions))
	for _, sub := range subscriptions {
		userIDs = append(userIDs, sub.UserID)
	}

	// Get all active devices for these users
	devices, err := s.subscriptionRepo.FindDevicesForUsers(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch devices: %w", err)
	}

	// If no devices, return early
	if len(devices) == 0 {
		return notification, nil
	}

	// Collect FCM tokens
	tokens := make([]string, 0, len(devices))
	deviceMap := make(map[string]*entity.UserDevice) // token -> device mapping
	for _, device := range devices {
		tokens = append(tokens, device.FCMToken)
		deviceMap[device.FCMToken] = device
	}

	// Prepare notification content
	title := "商戶位置通知"
	body := fmt.Sprintf("%s 已在 %s 開始營業", locationName, fullAddress)
	if hintMessage != "" {
		body = fmt.Sprintf("%s - %s", body, hintMessage)
	}

	notificationData := map[string]string{
		"notification_id": notification.ID.String(),
		"merchant_id":     merchantID.String(),
		"latitude":        fmt.Sprintf("%f", latitude),
		"longitude":       fmt.Sprintf("%f", longitude),
		"location_name":   locationName,
		"full_address":    fullAddress,
	}

	// Send notifications in batches
	var (
		totalSent        = 0
		totalFailed      = 0
		invalidTokens    []string
		notificationLogs []*entity.NotificationLog
	)

	for i := 0; i < len(tokens); i += firebaseBatchSize {
		end := i + firebaseBatchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		batch := tokens[i:end]

		// Send batch notification
		successCount, failureCount, batchInvalidTokens, err := s.notificationSvc.SendBatchNotification(
			ctx,
			batch,
			title,
			body,
			notificationData,
		)

		if err != nil {
			// Log error but continue with other batches
			totalFailed += len(batch)
			continue
		}

		totalSent += successCount
		totalFailed += failureCount
		invalidTokens = append(invalidTokens, batchInvalidTokens...)

		// Create notification logs for this batch
		for _, token := range batch {
			device := deviceMap[token]
			status := "sent"
			errorMsg := ""

			// Check if token is invalid
			for _, invalidToken := range batchInvalidTokens {
				if token == invalidToken {
					status = "failed"
					errorMsg = "invalid or unregistered token"
					break
				}
			}

			log := &entity.NotificationLog{
				ID:             uuid.New(),
				NotificationID: notification.ID,
				UserID:         device.UserID,
				DeviceID:       device.ID,
				Status:         status,
				FCMMessageID:   "", // Firebase doesn't provide individual message IDs in batch mode
				ErrorMessage:   errorMsg,
				SentAt:         time.Now(),
			}
			notificationLogs = append(notificationLogs, log)
		}
	}

	// Batch create notification logs
	if len(notificationLogs) > 0 {
		if err := s.notificationRepo.BatchCreateNotificationLogs(ctx, notificationLogs); err != nil {
			// Log error but don't fail the entire operation
			fmt.Printf("failed to create notification logs: %v\n", err)
		}
	}

	// Handle invalid tokens - soft delete devices
	if len(invalidTokens) > 0 {
		for _, token := range invalidTokens {
			if device, ok := deviceMap[token]; ok {
				if err := s.deviceRepo.DeleteDevice(ctx, device.ID); err != nil {
					// Log error but continue
					fmt.Printf("failed to delete invalid device %s: %v\n", device.ID, err)
				}
			}
		}
	}

	// Update notification statistics
	if err := s.notificationRepo.UpdateNotificationStatus(ctx, notification.ID, totalSent, totalFailed); err != nil {
		return nil, fmt.Errorf("failed to update notification status: %w", err)
	}

	// Update notification object
	notification.TotalSent = totalSent
	notification.TotalFailed = totalFailed

	return notification, nil
}

// GetMerchantNotificationHistory retrieves notification history for a merchant with pagination
func (s *notificationService) GetMerchantNotificationHistory(
	ctx context.Context,
	merchantID uuid.UUID,
	limit, offset int,
) ([]*entity.MerchantLocationNotification, error) {
	return s.notificationRepo.FindNotificationsByMerchant(ctx, merchantID, limit, offset)
}
