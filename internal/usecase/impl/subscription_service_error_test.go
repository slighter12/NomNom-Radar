package impl

import (
	"context"
	"testing"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSubscriptionService_SubscribeToMerchant_FindError(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, errors.New("db error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	assert.Error(t, err)
	assert.Nil(t, subscription)
}

func TestSubscriptionService_UnsubscribeFromMerchant_NotFound(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	err := fx.service.UnsubscribeFromMerchant(ctx, userID, merchantID)
	assert.Error(t, err)
	assert.Equal(t, ErrSubscriptionNotFound, err)
}

func TestSubscriptionService_ProcessQRSubscription_InvalidQR(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	qrData := "invalid-qr-data"

	fx.qrService.EXPECT().
		ParseSubscriptionQR(qrData).
		Return(uuid.Nil, errors.New("parse error"))

	subscription, err := fx.service.ProcessQRSubscription(ctx, userID, qrData, nil)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Equal(t, ErrInvalidQRCode, err)
}

func TestSubscriptionService_GetUserSubscriptions_Error(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionsByUser(ctx, userID).
		Return(nil, errors.New("database error"))

	subs, err := fx.service.GetUserSubscriptions(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, subs)
	assert.Contains(t, err.Error(), "failed to find subscriptions by user")
}

func TestSubscriptionService_GetMerchantSubscribers_Error(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	merchantID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionsByMerchant(ctx, merchantID).
		Return(nil, errors.New("database error"))

	subs, err := fx.service.GetMerchantSubscribers(ctx, merchantID)
	assert.Error(t, err)
	assert.Nil(t, subs)
	assert.Contains(t, err.Error(), "failed to find subscriptions by merchant")
}

func TestSubscriptionService_GenerateSubscriptionQR_Error(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	merchantID := uuid.New()

	fx.qrService.EXPECT().
		GenerateSubscriptionQR(merchantID).
		Return(nil, errors.New("qr generation failed"))

	qrCode, err := fx.service.GenerateSubscriptionQR(ctx, merchantID)
	assert.Error(t, err)
	assert.Nil(t, qrCode)
	assert.Contains(t, err.Error(), "failed to generate subscription QR")
}

func TestSubscriptionService_UnsubscribeFromMerchant_FindError(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, errors.New("database error"))

	err := fx.service.UnsubscribeFromMerchant(ctx, userID, merchantID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find subscription by user and merchant")
}

func TestSubscriptionService_UnsubscribeFromMerchant_DeleteError(t *testing.T) {
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
		Return(errors.New("database error"))

	err := fx.service.UnsubscribeFromMerchant(ctx, userID, merchantID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete subscription")
}

func TestSubscriptionService_SubscribeToMerchant_CreateError(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(nil, repository.ErrSubscriptionNotFound)

	fx.subRepo.EXPECT().
		CreateSubscription(ctx, mock.AnythingOfType("*entity.UserMerchantSubscription")).
		Return(errors.New("database error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Contains(t, err.Error(), "failed to create subscription")
}

func TestSubscriptionService_SubscribeToMerchant_WithDevice_FindDevicesError(t *testing.T) {
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
		Return(nil, errors.New("database error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Contains(t, err.Error(), "failed to find devices by user")
}

func TestSubscriptionService_SubscribeToMerchant_WithDevice_UpdateTokenError(t *testing.T) {
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
		Return(errors.New("database error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Contains(t, err.Error(), "failed to update FCM token")
}

func TestSubscriptionService_SubscribeToMerchant_WithDevice_CreateDeviceError(t *testing.T) {
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
		Return(errors.New("database error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Contains(t, err.Error(), "failed to create device")
}

func TestSubscriptionService_ReactivateSubscription_UpdateStatusError(t *testing.T) {
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
		Return(errors.New("database error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, nil)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Contains(t, err.Error(), "failed to update subscription status")
}

func TestSubscriptionService_ReactivateSubscription_WithDevice_FindError(t *testing.T) {
	fx := createTestSubscriptionService(t)

	ctx := context.Background()
	userID := uuid.New()
	merchantID := uuid.New()
	subID := uuid.New()
	deviceInfo := &usecase.DeviceInfo{
		FCMToken: "test-token",
		DeviceID: "device-123",
		Platform: "ios",
	}

	existingSub := &entity.UserMerchantSubscription{
		ID:         subID,
		UserID:     userID,
		MerchantID: merchantID,
		IsActive:   true, // Already active, no need to update status
	}

	fx.subRepo.EXPECT().
		FindSubscriptionByUserAndMerchant(ctx, userID, merchantID).
		Return(existingSub, nil)

	fx.deviceRepo.EXPECT().
		FindDevicesByUser(ctx, userID).
		Return(nil, errors.New("database error"))

	subscription, err := fx.service.SubscribeToMerchant(ctx, userID, merchantID, deviceInfo)
	assert.Error(t, err)
	assert.Nil(t, subscription)
	assert.Contains(t, err.Error(), "failed to find devices by user")
}
