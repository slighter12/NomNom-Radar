package notification

import (
	"context"
	"fmt"

	"radar/internal/domain/service"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type firebaseService struct {
	client *messaging.Client
}

// NewFirebaseService creates a new Firebase notification service instance
func NewFirebaseService(ctx context.Context, credentialsPath string) (service.NotificationService, error) {
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	return &firebaseService{
		client: client,
	}, nil
}

// SendSingleNotification sends a push notification to a single device token
func (s *firebaseService) SendSingleNotification(ctx context.Context, token, title, body string, data map[string]string) error {
	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	_, err := s.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// SendBatchNotification sends push notifications to multiple device tokens (max 500 tokens)
func (s *firebaseService) SendBatchNotification(ctx context.Context, tokens []string, title, body string, data map[string]string) (successCount, failureCount int, invalidTokens []string, err error) {
	if len(tokens) == 0 {
		return 0, 0, nil, nil
	}

	// Firebase limits to 500 tokens per request
	if len(tokens) > 500 {
		return 0, 0, nil, fmt.Errorf("token count exceeds limit: %d (max 500)", len(tokens))
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	response, err := s.client.SendEachForMulticast(ctx, message)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to send multicast notification: %w", err)
	}

	successCount = response.SuccessCount
	failureCount = response.FailureCount

	// Collect invalid tokens
	invalidTokens = make([]string, 0)
	for idx, sendResponse := range response.Responses {
		if sendResponse.Error != nil {
			// Check if error is due to invalid or unregistered token
			if messaging.IsInvalidArgument(sendResponse.Error) ||
				messaging.IsUnregistered(sendResponse.Error) {
				invalidTokens = append(invalidTokens, tokens[idx])
			}
		}
	}

	return successCount, failureCount, invalidTokens, nil
}
