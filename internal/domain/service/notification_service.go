package service

import (
	"context"
)

// NotificationService defines the interface for push notification services
type NotificationService interface {
	// SendBatchNotification sends push notifications to multiple device tokens
	// Returns success count, failure count, list of invalid tokens, and error
	SendBatchNotification(ctx context.Context, tokens []string, title, body string, data map[string]string) (successCount, failureCount int, invalidTokens []string, err error)

	// SendSingleNotification sends a push notification to a single device token
	SendSingleNotification(ctx context.Context, token, title, body string, data map[string]string) error
}
