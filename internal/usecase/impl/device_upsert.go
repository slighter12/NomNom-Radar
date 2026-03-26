package impl

import (
	"context"
	"errors"
	"fmt"
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

	device, err := deviceRepo.FindDeviceByUserAndDeviceID(ctx, userID, deviceInfo.DeviceID)
	if err == nil {
		if err := deviceRepo.UpdateFCMToken(ctx, device.ID, deviceInfo.FCMToken); err != nil {
			return nil, fmt.Errorf("failed to update FCM token: %w", err)
		}

		updatedDevice, err := deviceRepo.FindDeviceByID(ctx, device.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to find device by ID: %w", err)
		}

		return updatedDevice, nil
	}
	if !errors.Is(err, repository.ErrDeviceNotFound) {
		return nil, fmt.Errorf("failed to find device by user and device ID: %w", err)
	}

	newDevice := &entity.UserDevice{
		ID:        uuid.New(),
		UserID:    userID,
		FCMToken:  deviceInfo.FCMToken,
		DeviceID:  deviceInfo.DeviceID,
		Platform:  deviceInfo.Platform,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := deviceRepo.CreateDevice(ctx, newDevice); err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	return newDevice, nil
}

func validateDeviceInfo(deviceInfo *usecase.DeviceInfo) error {
	if deviceInfo == nil {
		return fmt.Errorf("device info is required: %w", domainerrors.ErrValidationFailed)
	}

	deviceInfo.FCMToken = strings.TrimSpace(deviceInfo.FCMToken)
	deviceInfo.DeviceID = strings.TrimSpace(deviceInfo.DeviceID)
	deviceInfo.Platform = strings.ToLower(strings.TrimSpace(deviceInfo.Platform))

	if deviceInfo.FCMToken == "" {
		return fmt.Errorf("fcm_token is required: %w", domainerrors.ErrValidationFailed)
	}
	if deviceInfo.DeviceID == "" {
		return fmt.Errorf("device_id is required: %w", domainerrors.ErrValidationFailed)
	}
	if deviceInfo.Platform != "ios" && deviceInfo.Platform != "android" {
		return fmt.Errorf("platform must be ios or android: %w", domainerrors.ErrValidationFailed)
	}

	return nil
}
