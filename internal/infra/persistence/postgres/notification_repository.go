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

// notificationRepository implements the repository.NotificationRepository interface.
type notificationRepository struct {
	q *query.Query
}

// NewNotificationRepository is the constructor for notificationRepository.
func NewNotificationRepository(db *gorm.DB) repository.NotificationRepository {
	return &notificationRepository{
		q: query.Use(db),
	}
}

// CreateNotification persists a new merchant location notification.
func (repo *notificationRepository) CreateNotification(ctx context.Context, notification *entity.MerchantLocationNotification) error {
	notificationM := fromNotificationDomain(notification)

	if err := repo.q.MerchantLocationNotificationModel.WithContext(ctx).Create(notificationM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid merchant or address reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required notification information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create notification")
	}

	// Update the entity with generated values
	notification.ID = notificationM.ID
	notification.PublishedAt = notificationM.PublishedAt
	notification.CreatedAt = notificationM.CreatedAt
	notification.UpdatedAt = notificationM.UpdatedAt

	return nil
}

// FindNotificationByID retrieves a notification by its unique ID.
func (repo *notificationRepository) FindNotificationByID(ctx context.Context, id uuid.UUID) (*entity.MerchantLocationNotification, error) {
	notificationM, err := repo.q.MerchantLocationNotificationModel.WithContext(ctx).
		Where(repo.q.MerchantLocationNotificationModel.ID.Eq(id)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotificationNotFound
		}

		return nil, errors.Wrap(err, "failed to find notification by ID")
	}

	return toNotificationDomain(notificationM), nil
}

// FindNotificationsByMerchant retrieves all notifications for a specific merchant with pagination.
func (repo *notificationRepository) FindNotificationsByMerchant(ctx context.Context, merchantID uuid.UUID, limit, offset int) ([]*entity.MerchantLocationNotification, error) {
	query := repo.q.MerchantLocationNotificationModel.WithContext(ctx).
		Where(repo.q.MerchantLocationNotificationModel.MerchantID.Eq(merchantID)).
		Order(repo.q.MerchantLocationNotificationModel.PublishedAt.Desc())

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	notificationModels, err := query.Find()
	if err != nil {
		return nil, errors.Wrap(err, "failed to find notifications by merchant")
	}

	notifications := make([]*entity.MerchantLocationNotification, 0, len(notificationModels))
	for _, notificationM := range notificationModels {
		notifications = append(notifications, toNotificationDomain(notificationM))
	}

	return notifications, nil
}

// UpdateNotificationStatus updates the total sent and failed counts for a notification.
func (repo *notificationRepository) UpdateNotificationStatus(ctx context.Context, id uuid.UUID, totalSent, totalFailed int) error {
	result, err := repo.q.MerchantLocationNotificationModel.WithContext(ctx).
		Where(repo.q.MerchantLocationNotificationModel.ID.Eq(id)).
		Updates(map[string]interface{}{
			"total_sent":   totalSent,
			"total_failed": totalFailed,
		})

	if err != nil {
		return errors.Wrap(err, "failed to update notification status")
	}

	if result.RowsAffected == 0 {
		return repository.ErrNotificationNotFound
	}

	return nil
}

// CreateNotificationLog persists a single notification log entry.
func (repo *notificationRepository) CreateNotificationLog(ctx context.Context, log *entity.NotificationLog) error {
	logM := fromNotificationLogDomain(log)

	if err := repo.q.NotificationLogModel.WithContext(ctx).Create(logM); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid notification, user, or device reference")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required notification log information")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to create notification log")
	}

	// Update the entity with generated values
	log.ID = logM.ID
	log.SentAt = logM.SentAt

	return nil
}

// BatchCreateNotificationLogs persists multiple notification log entries in a batch for better performance.
func (repo *notificationRepository) BatchCreateNotificationLogs(ctx context.Context, logs []*entity.NotificationLog) error {
	if len(logs) == 0 {
		return nil
	}

	logModels := make([]*model.NotificationLogModel, 0, len(logs))
	for _, log := range logs {
		logModels = append(logModels, fromNotificationLogDomain(log))
	}

	// Use GORM's CreateInBatches for efficient batch insertion
	// Default batch size is 100, which is a good balance between performance and memory
	if err := repo.q.NotificationLogModel.WithContext(ctx).CreateInBatches(logModels, 100); err != nil {
		// Convert PostgreSQL errors to domain errors
		if isForeignKeyConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("invalid notification, user, or device reference in batch")
		}
		if isNotNullConstraintViolation(err) {
			return domainerrors.ErrUserCreationFailed.WrapMessage("missing required notification log information in batch")
		}
		// For other database errors, return a generic database error
		return domainerrors.NewDatabaseExecuteError(err, "failed to batch create notification logs")
	}

	// Update the entities with generated values
	for i, logM := range logModels {
		logs[i].ID = logM.ID
		logs[i].SentAt = logM.SentAt
	}

	return nil
}

// --- Mapper Functions ---

// toNotificationDomain converts a GORM MerchantLocationNotificationModel to a domain MerchantLocationNotification entity.
func toNotificationDomain(data *model.MerchantLocationNotificationModel) *entity.MerchantLocationNotification {
	if data == nil {
		return nil
	}

	return &entity.MerchantLocationNotification{
		ID:           data.ID,
		MerchantID:   data.MerchantID,
		AddressID:    data.AddressID,
		LocationName: data.LocationName,
		FullAddress:  data.FullAddress,
		Latitude:     data.Latitude,
		Longitude:    data.Longitude,
		HintMessage:  data.HintMessage,
		TotalSent:    data.TotalSent,
		TotalFailed:  data.TotalFailed,
		PublishedAt:  data.PublishedAt,
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

// fromNotificationDomain converts a domain MerchantLocationNotification entity to a GORM MerchantLocationNotificationModel.
func fromNotificationDomain(data *entity.MerchantLocationNotification) *model.MerchantLocationNotificationModel {
	if data == nil {
		return nil
	}

	return &model.MerchantLocationNotificationModel{
		ID:           data.ID,
		MerchantID:   data.MerchantID,
		AddressID:    data.AddressID,
		LocationName: data.LocationName,
		FullAddress:  data.FullAddress,
		Latitude:     data.Latitude,
		Longitude:    data.Longitude,
		HintMessage:  data.HintMessage,
		TotalSent:    data.TotalSent,
		TotalFailed:  data.TotalFailed,
		PublishedAt:  data.PublishedAt,
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}

// fromNotificationLogDomain converts a domain NotificationLog entity to a GORM NotificationLogModel.
func fromNotificationLogDomain(data *entity.NotificationLog) *model.NotificationLogModel {
	if data == nil {
		return nil
	}

	return &model.NotificationLogModel{
		ID:             data.ID,
		NotificationID: data.NotificationID,
		UserID:         data.UserID,
		DeviceID:       data.DeviceID,
		Status:         data.Status,
		FCMMessageID:   data.FCMMessageID,
		ErrorMessage:   data.ErrorMessage,
		SentAt:         data.SentAt,
	}
}
