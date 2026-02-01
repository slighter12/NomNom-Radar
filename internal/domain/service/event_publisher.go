package service

import (
	"context"
)

// NotificationEvent represents an event to be processed by the geo worker
type NotificationEvent struct {
	RequestID      string   `json:"request_id,omitempty"` // For distributed tracing
	NotificationID string   `json:"notification_id"`
	MerchantID     string   `json:"merchant_id"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	LocationName   string   `json:"location_name"`
	FullAddress    string   `json:"full_address"`
	HintMessage    string   `json:"hint_message,omitempty"`
	SubscriberIDs  []string `json:"subscriber_ids"` // Pre-filtered subscriber user IDs
}

// EventPublisher defines the interface for publishing events to a message queue
type EventPublisher interface {
	// PublishNotificationEvent publishes a notification event for async processing
	PublishNotificationEvent(ctx context.Context, event *NotificationEvent) error

	// Close releases any resources held by the publisher
	Close() error
}
