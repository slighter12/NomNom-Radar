package impl

import (
	"context"
	"errors"
	"strings"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
)

func upsertUserDevice(
	ctx context.Context,
	deviceRepo repository.DeviceRepository,
	userID uuid.UUID,
	deviceInfo *usecase.DeviceInfo,
) (*entity.UserDevice, error) {
	if err := validateDeviceInfo(deviceInfo); err != nil {
		return nil, err
	}

	if device, err := upsertExistingDevice(ctx, deviceRepo, userID, deviceInfo); err != nil || device != nil {
		return device, err
	}

	if device, err := restoreDeletedDevice(ctx, deviceRepo, userID, deviceInfo); err != nil || device != nil {
		return device, err
	}

	now := time.Now()
	newDevice := &entity.UserDevice{
		ID:               uuid.New(),
		UserID:           userID,
		FCMToken:         deviceInfo.FCMToken,
		DeviceID:         deviceInfo.DeviceID,
		Platform:         deviceInfo.Platform,
		IsActive:         true,
		TokenRefreshedAt: now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := deviceRepo.CreateDevice(ctx, newDevice); err != nil {
		return nil, err
	}

	return newDevice, nil
}

func upsertExistingDevice(
	ctx context.Context,
	deviceRepo repository.DeviceRepository,
	userID uuid.UUID,
	deviceInfo *usecase.DeviceInfo,
) (*entity.UserDevice, error) {
	device, err := deviceRepo.FindDeviceByUserAndDeviceID(ctx, userID, deviceInfo.DeviceID)
	if errors.Is(err, domainerrors.ErrDeviceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := deviceRepo.UpdateFCMToken(ctx, device.ID, deviceInfo.FCMToken); err != nil {
		return nil, err
	}
	if !device.IsActive {
		if err := deviceRepo.SetDeviceActive(ctx, device.ID, true); err != nil {
			return nil, err
		}
	}

	return deviceRepo.FindDeviceByID(ctx, device.ID)
}

func restoreDeletedDevice(
	ctx context.Context,
	deviceRepo repository.DeviceRepository,
	userID uuid.UUID,
	deviceInfo *usecase.DeviceInfo,
) (*entity.UserDevice, error) {
	deletedDevice, err := deviceRepo.FindDeviceByUserAndDeviceIDIncludingDeleted(ctx, userID, deviceInfo.DeviceID)
	if errors.Is(err, domainerrors.ErrDeviceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := deviceRepo.RestoreAndUpdateDevice(ctx, userID, deletedDevice.ID, deviceInfo.FCMToken); err != nil {
		return nil, err
	}

	return deviceRepo.FindDeviceByUserAndDeviceID(ctx, userID, deviceInfo.DeviceID)
}

func validateDeviceInfo(deviceInfo *usecase.DeviceInfo) error {
	if deviceInfo == nil {
		return domainerrors.ErrValidationFailed.WithDetails("device info is required")
	}

	deviceInfo.FCMToken = strings.TrimSpace(deviceInfo.FCMToken)
	deviceInfo.DeviceID = strings.TrimSpace(deviceInfo.DeviceID)
	deviceInfo.Platform = strings.ToLower(strings.TrimSpace(deviceInfo.Platform))

	if deviceInfo.FCMToken == "" {
		return domainerrors.ErrValidationFailed.WithDetails("fcm_token is required")
	}
	if deviceInfo.DeviceID == "" {
		return domainerrors.ErrValidationFailed.WithDetails("device_id is required")
	}
	if deviceInfo.Platform != "ios" && deviceInfo.Platform != "android" {
		return domainerrors.ErrValidationFailed.WithDetails("platform must be ios or android")
	}

	return nil
}
