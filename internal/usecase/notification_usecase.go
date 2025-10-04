package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// LocationData represents location information for publishing a notification
type LocationData struct {
	LocationName string  `json:"location_name"`
	FullAddress  string  `json:"full_address"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
}

// NotificationUsecase defines the interface for notification management use cases
type NotificationUsecase interface {
	// PublishLocationNotification publishes a location notification to nearby subscribers
	// Either addressID or locationData must be provided
	PublishLocationNotification(ctx context.Context, merchantID uuid.UUID, addressID *uuid.UUID, locationData *LocationData, hintMessage string) (*entity.MerchantLocationNotification, error)

	// GetMerchantNotificationHistory retrieves notification history for a merchant with pagination
	GetMerchantNotificationHistory(ctx context.Context, merchantID uuid.UUID, limit, offset int) ([]*entity.MerchantLocationNotification, error)
}
