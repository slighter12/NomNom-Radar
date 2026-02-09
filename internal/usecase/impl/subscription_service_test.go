package impl

import (
	"context"
	"testing"

	"radar/config"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	mockRepo "radar/internal/mocks/repository"
	mockSvc "radar/internal/mocks/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// subscriptionServiceFixtures holds all test dependencies for subscription service tests.
type subscriptionServiceFixtures struct {
	service    usecase.SubscriptionUsecase
	subRepo    *mockRepo.MockSubscriptionRepository
	deviceRepo *mockRepo.MockDeviceRepository
	qrService  *mockSvc.MockQRCodeService
}

func createTestSubscriptionService(t *testing.T) subscriptionServiceFixtures {
	subRepo := mockRepo.NewMockSubscriptionRepository(t)
	deviceRepo := mockRepo.NewMockDeviceRepository(t)
	qrService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			DefaultRadius: 1000.0,
		},
	}
	service := NewSubscriptionService(SubscriptionServiceParams{
		SubscriptionRepo: subRepo,
		DeviceRepo:       deviceRepo,
		QRCodeService:    qrService,
		Config:           cfg,
	})

	return subscriptionServiceFixtures{
		service:    service,
		subRepo:    subRepo,
		deviceRepo: deviceRepo,
		qrService:  qrService,
	}
}

func TestSubscriptionService_SubscribeToMerchant_NewSubscription(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	fx.subRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.Equal(t, userID, subscription.UserID)
	assert.Equal(t, merchantID, subscription.MerchantID)
	assert.True(t, subscription.IsActive)
	assert.Equal(t, float64(1000.0), subscription.NotificationRadius)
}

func TestSubscriptionService_SubscribeToMerchant_ReactivateExisting(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	subID := uuid.New()

	existingSub := &entity.UserMerchantSubscription{
		ID:         subID,
		UserID:     userID,
		MerchantID: merchantID,
		IsActive:   false,
	}

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(existingSub, nil)

	fx.subRepo.EXPECT().
		UpdateSubscriptionStatus(ctx, subID, true).
		Return(nil)

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.True(t, subscription.IsActive)
}

func TestSubscriptionService_SubscribeToMerchant_WithDevice(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	fx.subRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	fx.deviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{}, nil)

	fx.deviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(nil)

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
}

func TestSubscriptionService_UnsubscribeFromMerchant_Success(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	subID := uuid.New()

	existingSub := &entity.UserMerchantSubscription{
		ID:         subID,
		UserID:     userID,
		MerchantID: merchantID,
		IsActive:   true,
	}

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(existingSub, nil)

	fx.subRepo.EXPECT().
		DeleteSubscription(ctx, subID).
		Return(nil)

	err := fx.service.UnsubscribeFromMerchant(ctx, userID, merchantID)
	require.NoError(t, err)
}

func TestSubscriptionService_GetUserSubscriptions(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	expectedSubs := []*entity.UserMerchantSubscription{
		{ID: uuid.New(), UserID: userID, MerchantID: uuid.New()},
	}

	fx.subRepo.EXPECT().
		FindSubscriptionsByUser(ctx, userID).
		Return(expectedSubs, nil)

	subs, err := fx.service.GetUserSubscriptions(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedSubs, subs)
}

func TestSubscriptionService_GetMerchantSubscribers(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	expectedSubs := []*entity.UserMerchantSubscription{
		{ID: uuid.New(), UserID: uuid.New(), MerchantID: merchantID},
	}

	fx.subRepo.EXPECT().
		FindSubscriptionsByMerchant(ctx, merchantID).
		Return(expectedSubs, nil)

	subs, err := fx.service.GetMerchantSubscribers(ctx, merchantID)
	require.NoError(t, err)
	assert.Equal(t, expectedSubs, subs)
}

func TestSubscriptionService_GenerateSubscriptionQR(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	merchantID := uuid.New()
	expectedQR := []byte("qr-code-data")

	fx.qrService.EXPECT().
		GenerateSubscriptionQR(merchantID).
		Return(expectedQR, nil)

	qrCode, err := fx.service.GenerateSubscriptionQR(ctx, merchantID)
	require.NoError(t, err)
	assert.Equal(t, expectedQR, qrCode)
}

func TestSubscriptionService_ProcessQRSubscription_Success(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	qrData := "qr-data-string"

	fx.qrService.EXPECT().
		ParseSubscriptionQR(qrData).
		Return(merchantID, nil)

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	fx.subRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	subscription, err := fx.service.ProcessQRSubscription(ctx, userID, qrData, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.Equal(t, merchantID, subscription.MerchantID)
}

func TestSubscriptionService_SubscribeToMerchant_ExistingDeviceUpdate(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	deviceID := uuid.New()
	existingDevice := &entity.UserDevice{
		ID:       deviceID,
		UserID:   userID,
		DeviceID: "device-123",
		FCMToken: "old-token",
	}
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "new-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	fx.subRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	fx.deviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{existingDevice}, nil)

	fx.deviceRepo.EXPECT().
		UpdateFCMToken(ctx, deviceID, "new-token").
		Return(nil)

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
}

func TestSubscriptionService_ProcessQRSubscription_WithDevice(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	qrData := "qr-data-string"
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	fx.qrService.EXPECT().
		ParseSubscriptionQR(qrData).
		Return(merchantID, nil)

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	fx.subRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	fx.deviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{}, nil)

	fx.deviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(nil)

	subscription, err := fx.service.ProcessQRSubscription(ctx, userID, qrData, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.Equal(t, merchantID, subscription.MerchantID)
}
