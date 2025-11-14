package notification

import (
	"context"
	"fmt"
	"log/slog"

	"radar/config"
	"radar/internal/domain/service"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/fx"
	"google.golang.org/api/option"
)

type firebaseService struct {
	client *messaging.Client
}

// noopNotificationService is a no-op implementation of the NotificationService
type noopNotificationService struct{}

// NewFirebaseService creates a new Firebase notification service instance
func NewFirebaseService(params FirebaseDependencies) (service.NotificationService, error) {
	// Firebase is optional - skip if not configured
	if params.Config.Firebase == nil {
		params.Logger.Info("Firebase not configured, notification service will be disabled")

		return &noopNotificationService{}, nil
	}

	// Validate Firebase configuration
	if params.Config.Firebase.ProjectID == "" || params.Config.Firebase.ProjectID == "your-project-id" {
		params.Logger.Warn("Firebase project ID not configured, notification service will be disabled")

		return &noopNotificationService{}, nil
	}

	if params.Config.Firebase.CredentialsPath == "" || params.Config.Firebase.CredentialsPath == "/path/to/firebase-service-account.json" {
		params.Logger.Warn("Firebase credentials path not configured, notification service will be disabled")

		return &noopNotificationService{}, nil
	}

	config := &firebase.Config{
		ProjectID: params.Config.Firebase.ProjectID,
	}
	opt := option.WithCredentialsFile(params.Config.Firebase.CredentialsPath)
	app, err := firebase.NewApp(params.LC, config, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Messaging(params.LC)
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	params.Logger.Info("Firebase notification service initialized successfully")

	return &firebaseService{
		client: client,
	}, nil
}

// SendSingleNotification is a no-op for the disabled notification service
func (s *noopNotificationService) SendSingleNotification(ctx context.Context, token, title, body string, data map[string]string) error {
	return nil
}

// SendBatchNotification is a no-op for the disabled notification service
func (s *noopNotificationService) SendBatchNotification(ctx context.Context, tokens []string, title, body string, data map[string]string) (successCount, failureCount int, invalidTokens []string, err error) {
	return 0, 0, nil, nil
}

type FirebaseDependencies struct {
	fx.In
	LC     context.Context
	Config *config.Config
	Logger *slog.Logger
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
