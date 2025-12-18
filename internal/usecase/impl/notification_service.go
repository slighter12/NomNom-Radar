package impl

import (
	"context"
	"fmt"
	"time"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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

	// Get location information
	locationName, fullAddress, latitude, longitude, err := s.getLocationInfo(ctx, merchantID, addressID, locationData)
	if err != nil {
		return nil, err
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
		return nil, errors.Wrap(err, "failed to create notification")
	}

	// Get devices for subscribers
	tokens, deviceMap, err := s.getSubscriberDevices(ctx, merchantID, latitude, longitude)
	if err != nil {
		return nil, err
	}

	// If no devices, return early
	if len(tokens) == 0 {
		return notification, nil
	}

	// Send and process notifications
	if err := s.sendAndProcessNotifications(ctx, notification, tokens, deviceMap, locationName, fullAddress, hintMessage, merchantID, latitude, longitude); err != nil {
		return nil, err
	}

	return notification, nil
}

// GetMerchantNotificationHistory retrieves notification history for a merchant with pagination
func (s *notificationService) GetMerchantNotificationHistory(
	ctx context.Context,
	merchantID uuid.UUID,
	limit, offset int,
) ([]*entity.MerchantLocationNotification, error) {
	notifications, err := s.notificationRepo.FindNotificationsByMerchant(ctx, merchantID, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find notifications by merchant")
	}

	return notifications, nil
}

// getLocationInfo retrieves location information from either addressID or locationData
func (s *notificationService) getLocationInfo(
	ctx context.Context,
	merchantID uuid.UUID,
	addressID *uuid.UUID,
	locationData *usecase.LocationData,
) (locationName, fullAddress string, latitude, longitude float64, err error) {
	if addressID != nil {
		// Fetch address from repository
		address, fetchErr := s.addressRepo.FindAddressByID(ctx, *addressID)
		if fetchErr != nil {
			return "", "", 0, 0, errors.Wrap(fetchErr, "failed to fetch address")
		}

		// Verify ownership
		if address.OwnerID != merchantID || address.OwnerType != entity.OwnerTypeMerchantProfile {
			return "", "", 0, 0, errors.New("unauthorized: address does not belong to merchant")
		}

		return address.Label, address.FullAddress, address.Latitude, address.Longitude, nil
	}

	// Use provided location data
	return locationData.LocationName, locationData.FullAddress, locationData.Latitude, locationData.Longitude, nil
}

// prepareNotificationContent prepares the notification title and body
func (s *notificationService) prepareNotificationContent(locationName, fullAddress, hintMessage string) (title, body string) {
	title = "商戶位置通知"
	body = fmt.Sprintf("%s 已在 %s 開始營業", locationName, fullAddress)
	if hintMessage != "" {
		body = fmt.Sprintf("%s - %s", body, hintMessage)
	}

	return title, body
}

// sendNotificationBatches sends notifications in batches and returns statistics
func (s *notificationService) sendNotificationBatches(
	ctx context.Context,
	tokens []string,
	deviceMap map[string]*entity.UserDevice,
	title, body string,
	notificationData map[string]string,
	notificationID uuid.UUID,
) (totalSent, totalFailed int, notificationLogs []*entity.NotificationLog, invalidTokens []string) {
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
		batchLogs := s.createNotificationLogs(batch, deviceMap, batchInvalidTokens, notificationID)
		notificationLogs = append(notificationLogs, batchLogs...)
	}

	return totalSent, totalFailed, notificationLogs, invalidTokens
}

// createNotificationLogs creates notification logs for a batch of tokens
func (s *notificationService) createNotificationLogs(
	tokens []string,
	deviceMap map[string]*entity.UserDevice,
	invalidTokens []string,
	notificationID uuid.UUID,
) []*entity.NotificationLog {
	logs := make([]*entity.NotificationLog, 0, len(tokens))

	for _, token := range tokens {
		device := deviceMap[token]
		status := "sent"
		errorMsg := ""

		// Check if token is invalid
		for _, invalidToken := range invalidTokens {
			if token == invalidToken {
				status = "failed"
				errorMsg = "invalid or unregistered token"

				break
			}
		}

		log := &entity.NotificationLog{
			ID:             uuid.New(),
			NotificationID: notificationID,
			UserID:         device.UserID,
			DeviceID:       device.ID,
			Status:         status,
			FCMMessageID:   "", // Firebase doesn't provide individual message IDs in batch mode
			ErrorMessage:   errorMsg,
			SentAt:         time.Now(),
		}
		logs = append(logs, log)
	}

	return logs
}

// handleInvalidTokens soft deletes devices with invalid tokens
func (s *notificationService) handleInvalidTokens(ctx context.Context, invalidTokens []string, deviceMap map[string]*entity.UserDevice) {
	for _, token := range invalidTokens {
		if device, ok := deviceMap[token]; ok {
			if err := s.deviceRepo.DeleteDevice(ctx, device.ID); err != nil {
				// Log error but continue
				fmt.Printf("failed to delete invalid device %s: %v\n", device.ID, err)
			}
		}
	}
}

// getSubscriberDevices retrieves devices for subscribers within radius
func (s *notificationService) getSubscriberDevices(
	ctx context.Context,
	merchantID uuid.UUID,
	latitude, longitude float64,
) (tokens []string, deviceMap map[string]*entity.UserDevice, err error) {
	// Find subscribers within radius using PostGIS
	subscriptions, err := s.subscriptionRepo.FindSubscribersWithinRadius(ctx, merchantID, latitude, longitude)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to find subscribers")
	}

	// If no subscribers, return early
	if len(subscriptions) == 0 {
		return []string{}, make(map[string]*entity.UserDevice), nil
	}

	// Collect user IDs
	userIDs := make([]uuid.UUID, 0, len(subscriptions))
	for _, sub := range subscriptions {
		userIDs = append(userIDs, sub.UserID)
	}

	// Get all active devices for these users
	devices, err := s.subscriptionRepo.FindDevicesForUsers(ctx, userIDs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch devices")
	}

	// If no devices, return early
	if len(devices) == 0 {
		return []string{}, make(map[string]*entity.UserDevice), nil
	}

	// Collect FCM tokens
	tokens = make([]string, 0, len(devices))
	deviceMap = make(map[string]*entity.UserDevice) // token -> device mapping
	for _, device := range devices {
		tokens = append(tokens, device.FCMToken)
		deviceMap[device.FCMToken] = device
	}

	return tokens, deviceMap, nil
}

// sendAndProcessNotifications sends notifications and processes the results
func (s *notificationService) sendAndProcessNotifications(
	ctx context.Context,
	notification *entity.MerchantLocationNotification,
	tokens []string,
	deviceMap map[string]*entity.UserDevice,
	locationName, fullAddress, hintMessage string,
	merchantID uuid.UUID,
	latitude, longitude float64,
) error {
	// Prepare notification content
	title, body := s.prepareNotificationContent(locationName, fullAddress, hintMessage)

	notificationData := map[string]string{
		"notification_id": notification.ID.String(),
		"merchant_id":     merchantID.String(),
		"latitude":        fmt.Sprintf("%f", latitude),
		"longitude":       fmt.Sprintf("%f", longitude),
		"location_name":   locationName,
		"full_address":    fullAddress,
	}

	// Send notifications in batches
	totalSent, totalFailed, notificationLogs, invalidTokens := s.sendNotificationBatches(
		ctx,
		tokens,
		deviceMap,
		title,
		body,
		notificationData,
		notification.ID,
	)

	// Batch create notification logs
	if len(notificationLogs) > 0 {
		if err := s.notificationRepo.BatchCreateNotificationLogs(ctx, notificationLogs); err != nil {
			// Log error but don't fail the entire operation
			fmt.Printf("failed to create notification logs: %v\n", err)
		}
	}

	// Handle invalid tokens - soft delete devices
	if len(invalidTokens) > 0 {
		s.handleInvalidTokens(ctx, invalidTokens, deviceMap)
	}

	// Update notification statistics
	if err := s.notificationRepo.UpdateNotificationStatus(ctx, notification.ID, totalSent, totalFailed); err != nil {
		return errors.Wrap(err, "failed to update notification status")
	}

	// Update notification object
	notification.TotalSent = totalSent
	notification.TotalFailed = totalFailed

	return nil
}
