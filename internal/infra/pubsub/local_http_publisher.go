package pubsub

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"radar/internal/domain/service"

	"github.com/pkg/errors"
)

// localHTTPPublisher implements EventPublisher by sending HTTP POST requests
// to a local endpoint, simulating Pub/Sub push behavior for development
type localHTTPPublisher struct {
	endpoint   string
	httpClient *http.Client
	logger     *slog.Logger
}

// PubSubPushMessage represents the structure of a Pub/Sub push message
// This mimics the format Google Pub/Sub uses when pushing to HTTP endpoints
type PubSubPushMessage struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes,omitempty"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// NewLocalHTTPPublisher creates a new local HTTP publisher for development
func NewLocalHTTPPublisher(endpoint string, logger *slog.Logger) service.EventPublisher {
	return &localHTTPPublisher{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// PublishNotificationEvent publishes an event by sending HTTP POST to the local endpoint
func (p *localHTTPPublisher) PublishNotificationEvent(ctx context.Context, event *service.NotificationEvent) error {
	// Serialize the event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return errors.WithStack(err)
	}

	// Create a Pub/Sub push message structure
	pushMsg := PubSubPushMessage{
		Subscription: "projects/local/subscriptions/notification-sub",
	}
	pushMsg.Message.Data = base64.StdEncoding.EncodeToString(eventData)
	pushMsg.Message.MessageID = event.NotificationID
	pushMsg.Message.PublishTime = time.Now().UTC().Format(time.RFC3339)

	// Build attributes with optional request_id for tracing
	attributes := map[string]string{
		"notification_id": event.NotificationID,
		"merchant_id":     event.MerchantID,
	}
	if event.RequestID != "" {
		attributes["request_id"] = event.RequestID
	}
	pushMsg.Message.Attributes = attributes

	// Serialize the push message
	body, err := json.Marshal(pushMsg)
	if err != nil {
		return errors.WithStack(err)
	}

	p.logger.Info("[LocalPubSub] Publishing event",
		slog.String("endpoint", p.endpoint),
		slog.String("notification_id", event.NotificationID),
		slog.Int("subscriber_count", len(event.SubscriberIDs)),
	)

	// Send HTTP POST request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.WithStack(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add X-Request-Id header for tracing
	if event.RequestID != "" {
		req.Header.Set("X-Request-Id", event.RequestID)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.Errorf("worker returned non-success status: %d", resp.StatusCode)
	}

	p.logger.Info("[LocalPubSub] Event published successfully",
		slog.String("notification_id", event.NotificationID),
	)

	return nil
}

// Close releases resources (no-op for HTTP client)
func (p *localHTTPPublisher) Close() error {
	return nil
}
