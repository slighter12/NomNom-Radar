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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionService_SubscribeToMerchant_NewSubscription(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			DefaultRadius: 1000.0,
		},
	}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	mockSubRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	subscription, err := service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.Equal(t, userID, subscription.UserID)
	assert.Equal(t, merchantID, subscription.MerchantID)
	assert.True(t, subscription.IsActive)
	assert.Equal(t, float64(1000.0), subscription.NotificationRadius)
}

func TestSubscriptionService_SubscribeToMerchant_ReactivateExisting(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			DefaultRadius: 1000.0,
		},
	}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

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

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(existingSub, nil)

	mockSubRepo.EXPECT().
		UpdateSubscriptionStatus(ctx, subID, true).
		Return(nil)

	subscription, err := service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.True(t, subscription.IsActive)
}

func TestSubscriptionService_SubscribeToMerchant_WithDevice(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			DefaultRadius: 1000.0,
		},
	}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	mockSubRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	mockDeviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return([]*entity.UserDevice{}, nil)

	mockDeviceRepo.EXPECT().
		CreateDevice(ctx, mock.AnythingOfType("*entity.UserDevice")).
		Return(nil)

	subscription, err := service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
}

func TestSubscriptionService_SubscribeToMerchant_FindError(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, errors.New("db error"))

	subscription, err := service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	assert.Error(t, err)
	assert.Nil(t, subscription)
}

func TestSubscriptionService_UnsubscribeFromMerchant_Success(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

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

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(existingSub, nil)

	mockSubRepo.EXPECT().
		DeleteSubscription(ctx, subID).
		Return(nil)

	err := service.UnsubscribeFromMerchant(ctx, userID, merchantID)
	require.NoError(t, err)
}

func TestSubscriptionService_UnsubscribeFromMerchant_NotFound(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	err := service.UnsubscribeFromMerchant(ctx, userID, merchantID)
	assert.Error(t, err)
	assert.Equal(t, ErrSubscriptionNotFound, err)
}

func TestSubscriptionService_GetUserSubscriptions(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	expectedSubs := []*entity.UserMerchantSubscription{
		{ID: uuid.New(), UserID: userID, MerchantID: uuid.New()},
	}

	mockSubRepo.EXPECT().
		FindSubscriptionsByUser(ctx, userID).
		Return(expectedSubs, nil)

	subs, err := service.GetUserSubscriptions(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedSubs, subs)
}

func TestSubscriptionService_GetMerchantSubscribers(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	merchantID := uuid.New()
	expectedSubs := []*entity.UserMerchantSubscription{
		{ID: uuid.New(), UserID: uuid.New(), MerchantID: merchantID},
	}

	mockSubRepo.EXPECT().
		FindSubscriptionsByMerchant(ctx, merchantID).
		Return(expectedSubs, nil)

	subs, err := service.GetMerchantSubscribers(ctx, merchantID)
	require.NoError(t, err)
	assert.Equal(t, expectedSubs, subs)
}

func TestSubscriptionService_GenerateSubscriptionQR(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	merchantID := uuid.New()
	expectedQR := []byte("qr-code-data")

	mockQRService.EXPECT().
		GenerateSubscriptionQR(merchantID).
		Return(expectedQR, nil)

	qrCode, err := service.GenerateSubscriptionQR(ctx, merchantID)
	require.NoError(t, err)
	assert.Equal(t, expectedQR, qrCode)
}

func TestSubscriptionService_ProcessQRSubscription_Success(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{
		LocationNotification: &config.LocationNotificationConfig{
			DefaultRadius: 1000.0,
		},
	}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	qrData := "qr-data-string"

	mockQRService.EXPECT().
		ParseSubscriptionQR(qrData).
		Return(merchantID, nil)

	mockSubRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	mockSubRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(nil)

	subscription, err := service.ProcessQRSubscription(ctx, userID, qrData, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscription)
	assert.Equal(t, merchantID, subscription.MerchantID)
}

func TestSubscriptionService_ProcessQRSubscription_InvalidQR(t *testing.T) {
	mockSubRepo := mockRepo.NewMockSubscriptionRepository(t)
	mockDeviceRepo := mockRepo.NewMockDeviceRepository(t)
	mockQRService := mockSvc.NewMockQRCodeService(t)
	cfg := &config.Config{}
	service := NewSubscriptionService(mockSubRepo, mockDeviceRepo, mockQRService, cfg)

	ctx := context.Background()
	userID := uuid.New()
	qrData := "invalid-qr-data"

	mockQRService.EXPECT().
		ParseSubscriptionQR(qrData).
		Return(uuid.Nil, errors.New("parse error"))

	subscription, err := service.ProcessQRSubscription(ctx, userID, qrData, nil)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Equal(t, ErrInvalidQRCode, err)
}
