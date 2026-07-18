package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	"radar/internal/infra/persistence/model"
	"radar/internal/infra/persistence/postgres/query"

	"github.com/google/uuid"
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
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrDeviceAlreadyExists
		}
		if isForeignKeyConstraintViolation(err) || isNotNullConstraintViolation(err) {
			return withSourceStack(domainerrors.ErrDeviceCreateFailed)
		}

		return withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	// Update the entity with generated values
	device.ID = deviceM.ID
	device.TokenRefreshedAt = deviceM.TokenRefreshedAt
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
			return nil, domainerrors.ErrDeviceNotFound
		}

		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return toDeviceDomain(deviceM), nil
}

// FindDeviceByUserAndDeviceID retrieves a device by user ID and client device ID.
func (repo *deviceRepository) FindDeviceByUserAndDeviceID(ctx context.Context, userID uuid.UUID, deviceID string) (*entity.UserDevice, error) {
	deviceM, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(
			repo.q.UserDeviceModel.UserID.Eq(userID),
			repo.q.UserDeviceModel.DeviceID.Eq(deviceID),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrDeviceNotFound
		}

		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return toDeviceDomain(deviceM), nil
}

// FindDevicesByUser retrieves devices for a specific user based on the provided filter.
func (repo *deviceRepository) FindDevicesByUser(
	ctx context.Context,
	userID uuid.UUID,
	filter repository.DeviceListFilter,
) ([]*entity.UserDevice, error) {
	queryDB := repo.q.UserDeviceModel.WithContext(ctx)
	if filter.IncludeDeleted || filter.OnlyDeleted {
		queryDB = queryDB.Unscoped()
	}

	queryDB = queryDB.Where(repo.q.UserDeviceModel.UserID.Eq(userID))

	if filter.OnlyHealthy {
		healthyWindowDays := filter.HealthyWindowDays
		if healthyWindowDays <= 0 {
			healthyWindowDays = policy.DefaultDevicePolicy().HealthyWindowDays
		}
		cutoff := time.Now().AddDate(0, 0, -healthyWindowDays)
		queryDB = queryDB.Where(
			repo.q.UserDeviceModel.IsActive.Is(true),
			repo.q.UserDeviceModel.DeletedAt.IsNull(),
			repo.q.UserDeviceModel.TokenRefreshedAt.Gt(cutoff),
		)
	} else if filter.OnlyDeleted {
		queryDB = queryDB.Not(repo.q.UserDeviceModel.DeletedAt.IsNull())
	}

	deviceModels, err := queryDB.Order(repo.q.UserDeviceModel.CreatedAt.Desc()).Find()
	if err != nil {
		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	devices := make([]*entity.UserDevice, 0, len(deviceModels))
	for _, deviceM := range deviceModels {
		devices = append(devices, toDeviceDomain(deviceM))
	}

	return devices, nil
}

// FindDeviceHealthByUser retrieves health projection fields for a user's devices, including soft-deleted records.
func (repo *deviceRepository) FindDeviceHealthByUser(ctx context.Context, userID uuid.UUID) ([]repository.DeviceHealthRecord, error) {
	deviceModels, err := repo.q.UserDeviceModel.WithContext(ctx).
		Unscoped().
		Where(repo.q.UserDeviceModel.UserID.Eq(userID)).
		Order(repo.q.UserDeviceModel.CreatedAt.Desc()).
		Find()
	if err != nil {
		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	records := make([]repository.DeviceHealthRecord, 0, len(deviceModels))
	for _, deviceM := range deviceModels {
		records = append(records, repository.DeviceHealthRecord{
			ID:               deviceM.ID,
			ClientDeviceID:   deviceM.DeviceID,
			TokenRefreshedAt: deviceM.TokenRefreshedAt,
			IsDeleted:        deviceM.DeletedAt.Valid,
		})
	}

	return records, nil
}

// FindDeviceByUserAndDeviceIDIncludingDeleted retrieves a device by user ID and client device ID, including soft-deleted records.
func (repo *deviceRepository) FindDeviceByUserAndDeviceIDIncludingDeleted(ctx context.Context, userID uuid.UUID, deviceID string) (*entity.UserDevice, error) {
	deviceM, err := repo.q.UserDeviceModel.WithContext(ctx).
		Unscoped().
		Where(
			repo.q.UserDeviceModel.UserID.Eq(userID),
			repo.q.UserDeviceModel.DeviceID.Eq(deviceID),
		).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainerrors.ErrDeviceNotFound
		}

		return nil, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return toDeviceDomain(deviceM), nil
}

// UpdateFCMToken updates the FCM token for a specific device.
func (repo *deviceRepository) UpdateFCMToken(ctx context.Context, deviceID uuid.UUID, fcmToken string) error {
	now := time.Now()
	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.ID.Eq(deviceID)).
		UpdateSimple(
			repo.q.UserDeviceModel.FCMToken.Value(fcmToken),
			repo.q.UserDeviceModel.TokenRefreshedAt.Value(now),
		)

	if err != nil {
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrDeviceAlreadyExists
		}

		return withSourceStack(domainerrors.ErrDeviceUpdateFailed)
	}

	if result.RowsAffected == 0 {
		return domainerrors.ErrDeviceNotFound
	}

	return nil
}

// SetDeviceActive updates the active state for a specific device without deleting it.
func (repo *deviceRepository) SetDeviceActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.ID.Eq(id)).
		Update(repo.q.UserDeviceModel.IsActive, isActive)

	if err != nil {
		return withSourceStack(domainerrors.ErrDeviceUpdateFailed)
	}

	if result.RowsAffected == 0 {
		return domainerrors.ErrDeviceNotFound
	}

	return nil
}

// RestoreAndUpdateDevice restores a soft-deleted device owned by the user and refreshes its token state.
func (repo *deviceRepository) RestoreAndUpdateDevice(ctx context.Context, userID, deviceID uuid.UUID, fcmToken string) error {
	now := time.Now()
	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Unscoped().
		Where(
			repo.q.UserDeviceModel.UserID.Eq(userID),
			repo.q.UserDeviceModel.ID.Eq(deviceID),
		).
		UpdateSimple(
			repo.q.UserDeviceModel.DeletedAt.Null(),
			repo.q.UserDeviceModel.FCMToken.Value(fcmToken),
			repo.q.UserDeviceModel.TokenRefreshedAt.Value(now),
			repo.q.UserDeviceModel.IsActive.Value(true),
		)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return domainerrors.ErrDeviceAlreadyExists
		}

		return withSourceStack(domainerrors.ErrDeviceUpdateFailed)
	}

	if result.RowsAffected == 0 {
		return domainerrors.ErrDeviceNotFound
	}

	return nil
}

// SoftDeleteStaleDevices soft-deletes devices with stale token refresh timestamps.
func (repo *deviceRepository) SoftDeleteStaleDevices(ctx context.Context, staleDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -staleDays)
	now := time.Now()

	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(
			repo.q.UserDeviceModel.DeletedAt.IsNull(),
			repo.q.UserDeviceModel.TokenRefreshedAt.Lte(cutoff),
		).
		UpdateSimple(
			repo.q.UserDeviceModel.DeletedAt.Value(sql.NullTime{Time: now, Valid: true}),
		)
	if err != nil {
		return 0, withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	return result.RowsAffected, nil
}

// DeleteDevice removes a device by its ID (soft delete).
func (repo *deviceRepository) DeleteDevice(ctx context.Context, id uuid.UUID) error {
	result, err := repo.q.UserDeviceModel.WithContext(ctx).
		Where(repo.q.UserDeviceModel.ID.Eq(id)).
		Delete()

	if err != nil {
		return withSourceStack(domainerrors.ErrPersistenceFailed)
	}

	if result.RowsAffected == 0 {
		return domainerrors.ErrDeviceNotFound
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
		ID:               data.ID,
		UserID:           data.UserID,
		FCMToken:         data.FCMToken,
		DeviceID:         data.DeviceID,
		Platform:         data.Platform,
		IsActive:         data.IsActive,
		TokenRefreshedAt: data.TokenRefreshedAt,
		CreatedAt:        data.CreatedAt,
		UpdatedAt:        data.UpdatedAt,
	}
}

// fromDeviceDomain converts a domain UserDevice entity to a GORM UserDeviceModel.
func fromDeviceDomain(data *entity.UserDevice) *model.UserDeviceModel {
	if data == nil {
		return nil
	}

	return &model.UserDeviceModel{
		ID:               data.ID,
		UserID:           data.UserID,
		FCMToken:         data.FCMToken,
		DeviceID:         data.DeviceID,
		Platform:         data.Platform,
		IsActive:         data.IsActive,
		TokenRefreshedAt: data.TokenRefreshedAt,
		CreatedAt:        data.CreatedAt,
		UpdatedAt:        data.UpdatedAt,
	}
}
