// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// deviceRepository implements the repository.DeviceRepository interface.
type deviceRepository struct {
	q *query.Query
}

// NewDeviceRepository is the constructor for deviceRepository.
func NewDeviceRepository(db *gorm.DB) repository.DeviceRepository {
	return &deviceRepository{
		q: query.Use(db),
	}
}

// CreateDevice persists a new device for a user.
func (repo *deviceRepository) CreateDevice(ctx context.Context, device *entity.UserDevice) error {
	deviceM := fromDeviceDomain(device)

	if err := repo.q.UserDeviceModel.WithContext(ctx).Create(deviceM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isUniqueConstraintViolation(err) {
			return repository.ErrDuplicateDevice
		}
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid user reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required device information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create device")
	}

	// Update the entity with generated values
	device.ID = deviceM.ID
	device.CreatedAt = deviceM.CreatedAt
	device.UpdatedAt = deviceM.UpdatedAt

	return nil
}

// FindDeviceByID retrieves a device by its unique ID.
func (repo *deviceRepository) FindDeviceByID(ctx context.Context, id uuid.UUID) (*entity.UserDevice, error) {
	deviceM, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.ID.Eq(id)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrDeviceNotFound
		}

		return nil, errors.Wrap(err, "failed to find device by ID")
	}

	return toDeviceDomain(deviceM), nil
}

// FindDevicesByUser retrieves all devices for a specific user (including inactive, excluding soft-deleted).
func (repo *deviceRepository) FindDevicesByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error) {
	deviceModels, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.UserID.Eq(userID)).
		Order(repo.q.UserDeviceModel.CreatedAt.Desc()).
		Find()

	if err != nil {
		return nil, errors.Wrap(err, "failed to find devices by user")
	}

	devices := make([]*entity.UserDevice, 0, len(deviceModels))
	for _, deviceM := range deviceModels {
		devices = append(devices, toDeviceDomain(deviceM))
	}

	return devices, nil
}

// FindActiveDevicesByUser retrieves all active devices for a specific user (excluding soft-deleted).
func (repo *deviceRepository) FindActiveDevicesByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserDevice, error) {
	deviceModels, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(
			repo.q.UserDeviceModel.UserID.Eq(userID),
			repo.q.UserDeviceModel.IsActive.Is(true),
		).
		Order(repo.q.UserDeviceModel.CreatedAt.Desc()).
		Find()

	if err != nil {
		return nil, errors.Wrap(err, "failed to find active devices by user")
	}

	devices := make([]*entity.UserDevice, 0, len(deviceModels))
	for _, deviceM := range deviceModels {
		devices = append(devices, toDeviceDomain(deviceM))
	}

	return devices, nil
}

// UpdateFCMToken updates the FCM token for a specific device.
func (repo *deviceRepository) UpdateFCMToken(ctx context.Context, deviceID uuid.UUID, fcmToken string) error {
	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.ID.Eq(deviceID)).
		Update(repo.q.UserDeviceModel.FCMToken, fcmToken)

	if err != nil {
		if isUniqueConstraintViolation(err) {
			return repository.ErrDuplicateDevice
		}

		return errors.Wrap(err, "failed to update FCM token")
	}

	if result.RowsAffected == 0 {
		return repository.ErrDeviceNotFound
	}

	return nil
}

// DeleteDevice removes a device by its ID (soft delete).
func (repo *deviceRepository) DeleteDevice(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return errors.Wrap(err, "failed to delete device")
	}

	if result.RowsAffected == 0 {
		return repository.ErrDeviceNotFound
	}

	return nil
}

// --- Mapper Functions ---

// toDeviceDomain converts a GORM UserDeviceModel to a domain UserDevice entity.
func toDeviceDomain(data *model.UserDeviceModel) *entity.UserDevice {
	if data == nil {
		return nil
	}

	return &entity.UserDevice{
		ID:        data.ID,
		UserID:    data.UserID,
		FCMToken:  data.FCMToken,
		DeviceID:  data.DeviceID,
		Platform:  data.Platform,
		IsActive:  data.IsActive,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}
}

// fromDeviceDomain converts a domain UserDevice entity to a GORM UserDeviceModel.
func fromDeviceDomain(data *entity.UserDevice) *model.UserDeviceModel {
	if data == nil {
		return nil
	}

	return &model.UserDeviceModel{
		ID:        data.ID,
		UserID:    data.UserID,
		FCMToken:  data.FCMToken,
		DeviceID:  data.DeviceID,
		Platform:  data.Platform,
		IsActive:  data.IsActive,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}
}
