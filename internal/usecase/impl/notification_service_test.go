package impl

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"
	mockRepo "radar/internal/mocks/repository"
	mockSvc "radar/internal/mocks/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestNotificationService(t *testing.T) (
	usecase.NotificationUsecase,
	*mockRepo.MockNotificationRepository,
	*mockRepo.MockSubscriptionRepository,
	*mockRepo.MockDeviceRepository,
	*mockRepo.MockAddressRepository,
	*mockSvc.MockNotificationService,
) {
	notificationRepo := mockRepo.NewMockNotificationRepository(t)
	subscriptionRepo := mockRepo.NewMockSubscriptionRepository(t)
	deviceRepo := mockRepo.NewMockDeviceRepository(t)
	addressRepo := mockRepo.NewMockAddressRepository(t)
	notificationSvc := mockSvc.NewMockNotificationService(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	service := NewNotificationService(
		logger,
		notificationRepo,
		subscriptionRepo,
		deviceRepo,
		addressRepo,
		notificationSvc,
		NewRoutingService(&config.RoutingConfig{
			MaxSnapDistanceKm: 1.0,
			DefaultSpeedKmh:   10.0,
			DataPath:          "./data/routing",
			Enabled:           true,
		}),
	)

	return service, notificationRepo, subscriptionRepo, deviceRepo, addressRepo, notificationSvc
}

func TestNotificationService_PublishLocationNotification_Success(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, notificationSvc := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{
		LocationName: "Test Store",
		FullAddress:  "123 Test St",
		Latitude:     25.0,
		Longitude:    121.0,
	}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "test-fcm-token"}
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"test-fcm-token"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 1, 0).Return(nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "hint")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 1, notification.TotalSent)
}

func TestNotificationService_PublishLocationNotification_NoSubscribers(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{
		LocationName: "Empty Store",
		Latitude:     25.0,
		Longitude:    121.0,
	}

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{}, nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.Equal(t, 0, notification.TotalSent)
}

func TestNotificationService_PublishLocationNotification_RoutingFailure(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, _ := createTestNotificationService(t)

	// Use a canceled context to trigger routing failure
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to trigger routing error

	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	_, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routing service failed")
}

func TestNotificationService_PublishLocationNotification_PartialDeliveryFailure(t *testing.T) {
	service, notificationRepo, subscriptionRepo, deviceRepo, _, notificationSvc := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	deviceID := uuid.New()
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{{ID: deviceID, UserID: subscriberOwnerID, FCMToken: "bad-token"}}, nil)

	// Simulate: 0 success, 1 failure with an invalid token that should be cleaned up
	notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"bad-token"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(0, 1, []string{"bad-token"}, nil)

	notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	deviceRepo.EXPECT().DeleteDevice(ctx, deviceID).Return(nil)
	notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 0, 1).Return(nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.Equal(t, 0, notification.TotalSent)
	assert.Equal(t, 1, notification.TotalFailed)
}

func TestNotificationService_PublishLocationNotification_InvalidInput(t *testing.T) {
	service, _, _, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, nil, "")

	assert.ErrorIs(t, err, ErrInvalidNotificationData)
	assert.Nil(t, notification)
}

func TestNotificationService_PublishLocationNotification_UnauthorizedAddress(t *testing.T) {
	service, _, _, _, addressRepo, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	otherMerchant := uuid.New()
	addressID := uuid.New()

	addressRepo.EXPECT().
		FindAddressByID(ctx, addressID).
		Return(&entity.Address{
			ID:        addressID,
			OwnerID:   otherMerchant,
			OwnerType: entity.OwnerTypeMerchantProfile,
			Label:     "Not owned",
		}, nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, &addressID, nil, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestNotificationService_PublishLocationNotification_SubscriptionError(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return(nil, errors.New("db error"))

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to find subscriber addresses")
}

func TestNotificationService_PublishLocationNotification_UnreachableTargets(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: uuid.New(), Latitude: 25.1, Longitude: 121.1}, NotificationRadius: 0.5},
		}, nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 0, notification.TotalSent)
}

func TestNotificationService_GetMerchantNotificationHistory(t *testing.T) {
	service, notificationRepo, _, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	expected := []*entity.MerchantLocationNotification{
		{ID: uuid.New(), MerchantID: merchantID},
	}

	notificationRepo.EXPECT().
		FindNotificationsByMerchant(ctx, merchantID, 10, 0).
		Return(expected, nil)

	got, err := service.GetMerchantNotificationHistory(ctx, merchantID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestNotificationService_PublishLocationNotification_WithAddressID(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, addressRepo, notificationSvc := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	addressID := uuid.New()
	subscriberOwnerID := uuid.New()

	address := &entity.Address{
		ID:          addressID,
		OwnerID:     merchantID,
		OwnerType:   entity.OwnerTypeMerchantProfile,
		Label:       "My Store",
		FullAddress: "456 Store Ave",
		Latitude:    25.05,
		Longitude:   121.05,
	}

	addressRepo.EXPECT().
		FindAddressByID(ctx, addressID).
		Return(address, nil)

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, address.Latitude, address.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.051, Longitude: 121.051}, NotificationRadius: 1.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "token-123"}
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"token-123"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 1, 0).Return(nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, &addressID, nil, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 1, notification.TotalSent)
	assert.Equal(t, "My Store", notification.LocationName)
	assert.Equal(t, "456 Store Ave", notification.FullAddress)
}

func TestNotificationService_PublishLocationNotification_AddressNotFound(t *testing.T) {
	service, _, _, _, addressRepo, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	addressID := uuid.New()

	addressRepo.EXPECT().
		FindAddressByID(ctx, addressID).
		Return(nil, errors.New("address not found"))

	notification, err := service.PublishLocationNotification(ctx, merchantID, &addressID, nil, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to fetch address")
}

func TestNotificationService_PublishLocationNotification_CreateNotificationError(t *testing.T) {
	service, notificationRepo, _, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	notificationRepo.EXPECT().
		CreateNotification(ctx, mock.Anything).
		Return(errors.New("db connection failed"))

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to create notification")
}

func TestNotificationService_PublishLocationNotification_FindDevicesError(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return(nil, errors.New("device query failed"))

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to fetch devices")
}

func TestNotificationService_PublishLocationNotification_SendBatchError(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, notificationSvc := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "token-xyz"}
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	// SendBatchNotification returns an error (e.g., Firebase service unavailable)
	notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"token-xyz"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(0, 0, nil, errors.New("firebase unavailable"))

	// Even with error, the flow continues and updates status with all failures
	notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 0, 1).Return(nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 0, notification.TotalSent)
	assert.Equal(t, 1, notification.TotalFailed)
}

func TestNotificationService_PublishLocationNotification_UpdateStatusError(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, notificationSvc := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "token-abc"}
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"token-abc"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	notificationRepo.EXPECT().
		UpdateNotificationStatus(ctx, mock.Anything, 1, 0).
		Return(errors.New("status update failed"))

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to update notification status")
}

func TestNotificationService_PublishLocationNotification_MultipleSubscribers(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, notificationSvc := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	user1ID := uuid.New()
	user2ID := uuid.New()
	user3ID := uuid.New()

	// 3 subscribers: 2 reachable (within radius), 1 unreachable (outside radius)
	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: user1ID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0}, // reachable
			{Address: entity.Address{OwnerID: user2ID, Latitude: 25.002, Longitude: 121.002}, NotificationRadius: 1.0}, // reachable
			{Address: entity.Address{OwnerID: user3ID, Latitude: 25.5, Longitude: 121.5}, NotificationRadius: 0.1},     // unreachable (too far)
		}, nil)

	// Only 2 users are reachable - use assert.ElementsMatch for unordered slice comparison
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, mock.MatchedBy(func(ids []uuid.UUID) bool {
			return assert.ElementsMatch(t, []uuid.UUID{user1ID, user2ID}, ids)
		})).
		Return([]*entity.UserDevice{
			{ID: uuid.New(), UserID: user1ID, FCMToken: "token-1"},
			{ID: uuid.New(), UserID: user2ID, FCMToken: "token-2"},
		}, nil)

	notificationSvc.EXPECT().
		SendBatchNotification(ctx, mock.MatchedBy(func(tokens []string) bool {
			return assert.ElementsMatch(t, []string{"token-1", "token-2"}, tokens)
		}), "商戶位置通知", mock.Anything, mock.Anything).
		Return(2, 0, nil, nil)

	notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 2, 0).Return(nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.Equal(t, 2, notification.TotalSent)
	assert.Equal(t, 0, notification.TotalFailed)
}

func TestNotificationService_PublishLocationNotification_NoDevicesForSubscribers(t *testing.T) {
	service, notificationRepo, subscriptionRepo, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1.0},
		}, nil)

	// Subscriber exists but has no registered devices
	subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{}, nil)

	notification, err := service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 0, notification.TotalSent)
	assert.Equal(t, 0, notification.TotalFailed)
}

func TestNotificationService_GetMerchantNotificationHistory_Error(t *testing.T) {
	service, notificationRepo, _, _, _, _ := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()

	notificationRepo.EXPECT().
		FindNotificationsByMerchant(ctx, merchantID, 10, 0).
		Return(nil, errors.New("database error"))

	notifications, err := service.GetMerchantNotificationHistory(ctx, merchantID, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, notifications)
	assert.Contains(t, err.Error(), "failed to find notifications by merchant")
}
