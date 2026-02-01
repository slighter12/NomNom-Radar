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

// notificationServiceFixtures holds all test dependencies for notification service tests.
type notificationServiceFixtures struct {
	service          usecase.NotificationUsecase
	notificationRepo *mockRepo.MockNotificationRepository
	subscriptionRepo *mockRepo.MockSubscriptionRepository
	deviceRepo       *mockRepo.MockDeviceRepository
	addressRepo      *mockRepo.MockAddressRepository
	notificationSvc  *mockSvc.MockNotificationService
}

func createTestNotificationService(t *testing.T) notificationServiceFixtures {
	notificationRepo := mockRepo.NewMockNotificationRepository(t)
	subscriptionRepo := mockRepo.NewMockSubscriptionRepository(t)
	deviceRepo := mockRepo.NewMockDeviceRepository(t)
	addressRepo := mockRepo.NewMockAddressRepository(t)
	notificationSvc := mockSvc.NewMockNotificationService(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	routingSvc := NewRoutingService(RoutingServiceParams{
		Config: &config.RoutingConfig{
			MaxSnapDistanceKm: 1.0,
			DefaultSpeedKmh:   10.0,
			DataPath:          "./data/routing",
			Enabled:           false, // Disabled to use Haversine fallback
		},
		Logger: nil,
	})

	service := NewNotificationService(NotificationServiceParams{
		Logger:           logger,
		NotificationRepo: notificationRepo,
		SubscriptionRepo: subscriptionRepo,
		DeviceRepo:       deviceRepo,
		AddressRepo:      addressRepo,
		NotificationSvc:  notificationSvc,
		RoutingSvc:       routingSvc,
	})

	return notificationServiceFixtures{
		service:          service,
		notificationRepo: notificationRepo,
		subscriptionRepo: subscriptionRepo,
		deviceRepo:       deviceRepo,
		addressRepo:      addressRepo,
		notificationSvc:  notificationSvc,
	}
}

func TestNotificationService_PublishLocationNotification_Success(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{
		LocationName: "Test Store",
		FullAddress:  "123 Test St",
		Latitude:     25.0,
		Longitude:    121.0,
	}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "test-fcm-token"}
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"test-fcm-token"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	fx.notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	fx.notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 1, 0).Return(nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "hint")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 1, notification.TotalSent)
}

func TestNotificationService_PublishLocationNotification_NoSubscribers(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{
		LocationName: "Empty Store",
		Latitude:     25.0,
		Longitude:    121.0,
	}

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{}, nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.Equal(t, 0, notification.TotalSent)
}

func TestNotificationService_PublishLocationNotification_RoutingFailure(t *testing.T) {
	fx := createTestNotificationService(t)

	// Use a canceled context to trigger routing failure
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to trigger routing error

	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	_, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routing service failed")
}

func TestNotificationService_PublishLocationNotification_PartialDeliveryFailure(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	deviceID := uuid.New()
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{{ID: deviceID, UserID: subscriberOwnerID, FCMToken: "bad-token"}}, nil)

	// Simulate: 0 success, 1 failure with an invalid token that should be cleaned up
	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"bad-token"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(0, 1, []string{"bad-token"}, nil)

	fx.notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	fx.deviceRepo.EXPECT().DeleteDevice(ctx, deviceID).Return(nil)
	fx.notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 0, 1).Return(nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.Equal(t, 0, notification.TotalSent)
	assert.Equal(t, 1, notification.TotalFailed)
}

func TestNotificationService_PublishLocationNotification_InvalidInput(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, nil, "")

	assert.ErrorIs(t, err, ErrInvalidNotificationData)
	assert.Nil(t, notification)
}

func TestNotificationService_PublishLocationNotification_UnauthorizedAddress(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	otherMerchant := uuid.New()
	addressID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, addressID).
		Return(&entity.Address{
			ID:        addressID,
			OwnerID:   otherMerchant,
			OwnerType: entity.OwnerTypeMerchantProfile,
			Label:     "Not owned",
		}, nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, &addressID, nil, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestNotificationService_PublishLocationNotification_SubscriptionError(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return(nil, errors.New("db error"))

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to find subscriber addresses")
}

func TestNotificationService_PublishLocationNotification_UnreachableTargets(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: uuid.New(), Latitude: 25.1, Longitude: 121.1}, NotificationRadius: 500.0},
		}, nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 0, notification.TotalSent)
}

func TestNotificationService_GetMerchantNotificationHistory(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	expected := []*entity.MerchantLocationNotification{
		{ID: uuid.New(), MerchantID: merchantID},
	}

	fx.notificationRepo.EXPECT().
		FindNotificationsByMerchant(ctx, merchantID, 10, 0).
		Return(expected, nil)

	got, err := fx.service.GetMerchantNotificationHistory(ctx, merchantID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestNotificationService_PublishLocationNotification_WithAddressID(t *testing.T) {
	fx := createTestNotificationService(t)

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

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, addressID).
		Return(address, nil)

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, address.Latitude, address.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.051, Longitude: 121.051}, NotificationRadius: 1000.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "token-123"}
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"token-123"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	fx.notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	fx.notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 1, 0).Return(nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, &addressID, nil, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 1, notification.TotalSent)
	assert.Equal(t, "My Store", notification.LocationName)
	assert.Equal(t, "456 Store Ave", notification.FullAddress)
}

func TestNotificationService_PublishLocationNotification_AddressNotFound(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	addressID := uuid.New()

	fx.addressRepo.EXPECT().
		FindAddressByID(ctx, addressID).
		Return(nil, errors.New("address not found"))

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, &addressID, nil, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to fetch address")
}

func TestNotificationService_PublishLocationNotification_CreateNotificationError(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	fx.notificationRepo.EXPECT().
		CreateNotification(ctx, mock.Anything).
		Return(errors.New("db connection failed"))

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to create notification")
}

func TestNotificationService_PublishLocationNotification_FindDevicesError(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return(nil, errors.New("device query failed"))

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to fetch devices")
}

func TestNotificationService_PublishLocationNotification_SendBatchError(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "token-xyz"}
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	// SendBatchNotification returns an error (e.g., Firebase service unavailable)
	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"token-xyz"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(0, 0, nil, errors.New("firebase unavailable"))

	// Even with error, the flow continues and updates status with all failures
	fx.notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 0, 1).Return(nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 0, notification.TotalSent)
	assert.Equal(t, 1, notification.TotalFailed)
}

func TestNotificationService_PublishLocationNotification_UpdateStatusError(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	userDevice := &entity.UserDevice{ID: uuid.New(), UserID: subscriberOwnerID, FCMToken: "token-abc"}
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{userDevice}, nil)

	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"token-abc"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	fx.notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	fx.notificationRepo.EXPECT().
		UpdateNotificationStatus(ctx, mock.Anything, 1, 0).
		Return(errors.New("status update failed"))

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	assert.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "failed to update notification status")
}

func TestNotificationService_PublishLocationNotification_MultipleSubscribers(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	user1ID := uuid.New()
	user2ID := uuid.New()
	user3ID := uuid.New()

	// 3 subscribers: 2 reachable (within radius), 1 unreachable (outside radius)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: user1ID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0}, // reachable
			{Address: entity.Address{OwnerID: user2ID, Latitude: 25.002, Longitude: 121.002}, NotificationRadius: 1000.0}, // reachable
			{Address: entity.Address{OwnerID: user3ID, Latitude: 25.5, Longitude: 121.5}, NotificationRadius: 100.0},     // unreachable (too far)
		}, nil)

	// Only 2 users are reachable - use assert.ElementsMatch for unordered slice comparison
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, mock.MatchedBy(func(ids []uuid.UUID) bool {
			return assert.ElementsMatch(t, []uuid.UUID{user1ID, user2ID}, ids)
		})).
		Return([]*entity.UserDevice{
			{ID: uuid.New(), UserID: user1ID, FCMToken: "token-1"},
			{ID: uuid.New(), UserID: user2ID, FCMToken: "token-2"},
		}, nil)

	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, mock.MatchedBy(func(tokens []string) bool {
			return assert.ElementsMatch(t, []string{"token-1", "token-2"}, tokens)
		}), "商戶位置通知", mock.Anything, mock.Anything).
		Return(2, 0, nil, nil)

	fx.notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	fx.notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 2, 0).Return(nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.Equal(t, 2, notification.TotalSent)
	assert.Equal(t, 0, notification.TotalFailed)
}

func TestNotificationService_PublishLocationNotification_NoDevicesForSubscribers(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	locationData := &usecase.LocationData{Latitude: 25.0, Longitude: 121.0}
	subscriberOwnerID := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{Address: entity.Address{OwnerID: subscriberOwnerID, Latitude: 25.001, Longitude: 121.001}, NotificationRadius: 1000.0},
		}, nil)

	// Subscriber exists but has no registered devices
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{subscriberOwnerID}).
		Return([]*entity.UserDevice{}, nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	assert.Equal(t, 0, notification.TotalSent)
	assert.Equal(t, 0, notification.TotalFailed)
}

func TestNotificationService_GetMerchantNotificationHistory_Error(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()

	fx.notificationRepo.EXPECT().
		FindNotificationsByMerchant(ctx, merchantID, 10, 0).
		Return(nil, errors.New("database error"))

	notifications, err := fx.service.GetMerchantNotificationHistory(ctx, merchantID, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, notifications)
	assert.Contains(t, err.Error(), "failed to find notifications by merchant")
}

// TestNotificationService_HaversineDistanceFiltering verifies that subscribers
// outside the notification radius are correctly filtered out using Haversine (straight-line) distance.
// Note: This test uses Haversine fallback since routing engine is disabled in the test fixture.
func TestNotificationService_HaversineDistanceFiltering(t *testing.T) {
	fx := createTestNotificationService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	// Merchant at Taipei
	locationData := &usecase.LocationData{
		LocationName: "Taipei Store",
		FullAddress:  "123 Taipei St",
		Latitude:     25.0330,
		Longitude:    121.5654,
	}

	nearbyOwner := uuid.New()
	farOwner := uuid.New()

	fx.notificationRepo.EXPECT().CreateNotification(ctx, mock.Anything).Return(nil)

	// Two subscribers: one nearby (within 1000m/1km), one far (beyond 5000m/5km)
	fx.subscriptionRepo.EXPECT().
		FindSubscriberAddressesWithinRadius(ctx, merchantID, locationData.Latitude, locationData.Longitude).
		Return([]*entity.SubscriberAddress{
			{
				Address:            entity.Address{OwnerID: nearbyOwner, Latitude: 25.0335, Longitude: 121.5660},
				NotificationRadius: 1000.0, // 1000m (1km) radius - should match
			},
			{
				Address:            entity.Address{OwnerID: farOwner, Latitude: 25.1, Longitude: 121.7},
				NotificationRadius: 500.0, // 500m (0.5km) radius - too far away
			},
		}, nil)

	// Only the nearby subscriber should have their device queried
	nearbyDevice := &entity.UserDevice{ID: uuid.New(), UserID: nearbyOwner, FCMToken: "nearby-token"}
	fx.subscriptionRepo.EXPECT().
		FindDevicesForUsers(ctx, []uuid.UUID{nearbyOwner}).
		Return([]*entity.UserDevice{nearbyDevice}, nil)

	fx.notificationSvc.EXPECT().
		SendBatchNotification(ctx, []string{"nearby-token"}, "商戶位置通知", mock.Anything, mock.Anything).
		Return(1, 0, nil, nil)

	fx.notificationRepo.EXPECT().BatchCreateNotificationLogs(ctx, mock.Anything).Return(nil)
	fx.notificationRepo.EXPECT().UpdateNotificationStatus(ctx, mock.Anything, 1, 0).Return(nil)

	notification, err := fx.service.PublishLocationNotification(ctx, merchantID, nil, locationData, "")

	require.NoError(t, err)
	assert.NotNil(t, notification)
	// Only 1 notification sent (to nearby subscriber), far subscriber was filtered out
	assert.Equal(t, 1, notification.TotalSent)
	assert.Equal(t, 0, notification.TotalFailed)
}
