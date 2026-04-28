package impl

import (
	"context"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

type deviceService struct {
	deviceRepo repository.DeviceRepository
}

// DeviceServiceParams holds dependencies for DeviceService, injected by Fx.
type DeviceServiceParams struct {
	fx.In

	DeviceRepo repository.DeviceRepository
}

// NewDeviceService creates a new device service instance
func NewDeviceService(params DeviceServiceParams) usecase.DeviceUsecase {
	return &deviceService{
		deviceRepo: params.DeviceRepo,
	}
}

// RegisterDevice registers a new device or updates an existing one
func (s *deviceService) RegisterDevice(ctx context.Context, userID uuid.UUID, deviceInfo *usecase.DeviceInfo) (*entity.UserDevice, error) {
	return upsertUserDevice(ctx, s.deviceRepo, userID, deviceInfo)
}

// UpdateFCMToken updates the FCM token for a specific device
func (s *deviceService) UpdateFCMToken(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, fcmToken string) error {
	if err := s.ensureOwnedDevice(ctx, userID, deviceID); err != nil {
		return err
	}

	if err := s.deviceRepo.UpdateFCMToken(ctx, deviceID, fcmToken); err != nil {
		return err
	}

	return nil
}

// GetUserDevices retrieves active devices with healthy push tokens for a user.
func (s *deviceService) GetUserDevices(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error) {
	devices, err := s.deviceRepo.FindDevicesByUser(ctx, userID, repository.DeviceListFilter{
		OnlyHealthy:       true,
		HealthyWindowDays: policy.DefaultDevicePolicy().HealthyWindowDays,
	})
	if err != nil {
		return nil, err
	}

	return devices, nil
}

// GetDeviceHealth retrieves computed health information for all user devices, including invalidated ones.
func (s *deviceService) GetDeviceHealth(ctx context.Context, userID uuid.UUID) ([]*usecase.DeviceHealthInfo, error) {
	records, err := s.deviceRepo.FindDeviceHealthByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -policy.DefaultDevicePolicy().HealthyWindowDays)
	result := make([]*usecase.DeviceHealthInfo, 0, len(records))

	for _, record := range records {
		result = append(result, buildDeviceHealthInfo(record, cutoff))
	}

	return result, nil
}

// DeactivateDevice deactivates a device without soft-deleting it.
func (s *deviceService) DeactivateDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	if err := s.ensureOwnedDevice(ctx, userID, deviceID); err != nil {
		return err
	}

	if err := s.deviceRepo.SetDeviceActive(ctx, deviceID, false); err != nil {
		return err
	}

	return nil
}

func buildDeviceHealthInfo(record repository.DeviceHealthRecord, cutoff time.Time) *usecase.DeviceHealthInfo {
	healthStatus := usecase.DeviceHealthStatusHealthy
	if record.IsDeleted {
		healthStatus = usecase.DeviceHealthStatusInvalid
	} else if !record.TokenRefreshedAt.After(cutoff) {
		healthStatus = usecase.DeviceHealthStatusStale
	}

	return &usecase.DeviceHealthInfo{
		ID:               record.ID,
		ClientDeviceID:   record.ClientDeviceID,
		HealthStatus:     healthStatus,
		TokenRefreshedAt: record.TokenRefreshedAt,
		RequiresRebind:   requiresDeviceRebind(healthStatus),
	}
}

func requiresDeviceRebind(status usecase.DeviceHealthStatus) bool {
	return status == usecase.DeviceHealthStatusStale || status == usecase.DeviceHealthStatusInvalid
}

func (s *deviceService) ensureOwnedDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	device, err := s.deviceRepo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		return err
	}

	if device.UserID != userID {
		return domainerrors.ErrDeviceOwnershipViolation
	}

	return nil
}
