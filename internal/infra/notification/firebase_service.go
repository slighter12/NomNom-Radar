package notification

import (
	"context"
	"log/slog"
	"os"

	"radar/config"
	"radar/internal/domain/service"
	"radar/internal/errors"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/fx"
	"google.golang.org/api/option"
)

type firebaseService struct {
	client *messaging.Client
}

// NewFirebaseService creates a new Firebase notification service instance
func NewFirebaseService(params FirebaseDependencies) (service.NotificationService, error) {
	if params.Config.Firebase == nil {
		return nil, errors.New("firebase config must be configured")
	}

	if params.Config.Firebase.ProjectID == "" || params.Config.Firebase.ProjectID == "your-project-id" {
		return nil, errors.New("firebase project ID must be configured")
	}

	if params.Config.Firebase.CredentialsPath == "" || params.Config.Firebase.CredentialsPath == "/path/to/firebase-service-account.json" {
		return nil, errors.New("firebase credentials path must be configured")
	}

	credentialsJSON, readErr := os.ReadFile(params.Config.Firebase.CredentialsPath)
	if readErr != nil {
		return nil, errors.Wrap(readErr, "failed to read firebase credentials")
	}

	config := &firebase.Config{
		ProjectID: params.Config.Firebase.ProjectID,
	}
	opt := option.WithAuthCredentialsJSON(option.ServiceAccount, credentialsJSON)
	app, err := firebase.NewApp(params.LC, config, opt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize firebase app")
	}

	client, err := app.Messaging(params.LC)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create firebase messaging client")
	}

	params.Logger.Info("Firebase notification service initialized successfully")

	return &firebaseService{
		client: client,
	}, nil
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
		return errors.WithStack(err)
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
		return 0, 0, nil, errors.Errorf("token count exceeds limit: %d (max 500)", len(tokens))
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
		return 0, 0, nil, errors.WithStack(err)
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
