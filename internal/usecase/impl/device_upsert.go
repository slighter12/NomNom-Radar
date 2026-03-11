package impl

import (
	"context"
	"strings"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/errors"
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

	device, err := deviceRepo.FindDeviceByUserAndDeviceID(ctx, userID, deviceInfo.DeviceID)
	if err == nil {
		if err := deviceRepo.UpdateFCMToken(ctx, device.ID, deviceInfo.FCMToken); err != nil {
			return nil, errors.Wrap(err, "failed to update FCM token")
		}

		updatedDevice, err := deviceRepo.FindDeviceByID(ctx, device.ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find device by ID")
		}

		return updatedDevice, nil
	}
	if !errors.Is(err, repository.ErrDeviceNotFound) {
		return nil, errors.Wrap(err, "failed to find device by user and device ID")
	}

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

	if err := deviceRepo.CreateDevice(ctx, device); err != nil {
		return nil, errors.Wrap(err, "failed to create device")
	}

	return device, nil
}

func validateDeviceInfo(deviceInfo *usecase.DeviceInfo) error {
	if deviceInfo == nil {
		return errors.Wrap(domainerrors.ErrValidationFailed, "device info is required")
	}

	deviceInfo.FCMToken = strings.TrimSpace(deviceInfo.FCMToken)
	deviceInfo.DeviceID = strings.TrimSpace(deviceInfo.DeviceID)
	deviceInfo.Platform = strings.ToLower(strings.TrimSpace(deviceInfo.Platform))

	if deviceInfo.FCMToken == "" {
		return errors.Wrap(domainerrors.ErrValidationFailed, "fcm_token is required")
	}
	if deviceInfo.DeviceID == "" {
		return errors.Wrap(domainerrors.ErrValidationFailed, "device_id is required")
	}
	if deviceInfo.Platform != "ios" && deviceInfo.Platform != "android" {
		return errors.Wrap(domainerrors.ErrValidationFailed, "platform must be ios or android")
	}

	return nil
}
