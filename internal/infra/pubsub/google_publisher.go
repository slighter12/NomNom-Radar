package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"radar/internal/domain/service"

	"cloud.google.com/go/pubsub/v2"
	pubsubpb "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/pkg/errors"
)

// googlePubSubPublisher implements EventPublisher using Google Cloud Pub/Sub
type googlePubSubPublisher struct {
	client    *pubsub.Client
	publisher *pubsub.Publisher
	logger    *slog.Logger
}

// NewGooglePubSubPublisher creates a new Google Pub/Sub publisher
func NewGooglePubSubPublisher(ctx context.Context, projectID, topicID string, logger *slog.Logger) (service.EventPublisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Check if topic exists using TopicAdminClient
	topicPath := fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
	_, err = client.TopicAdminClient.GetTopic(ctx, &pubsubpb.GetTopicRequest{
		Topic: topicPath,
	})
	if err != nil {
		client.Close()

		return nil, errors.Wrapf(err, "failed to get topic %s", topicID)
	}

	publisher := client.Publisher(topicID)

	logger.Info("Google Pub/Sub publisher initialized",
		slog.String("project_id", projectID),
		slog.String("topic_id", topicID),
	)

	return &googlePubSubPublisher{
		client:    client,
		publisher: publisher,
		logger:    logger,
	}, nil
}

// PublishNotificationEvent publishes an event to Google Pub/Sub
func (p *googlePubSubPublisher) PublishNotificationEvent(ctx context.Context, event *service.NotificationEvent) error {
	// Serialize the event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return errors.WithStack(err)
	}

	// Create Pub/Sub message with attributes for filtering and tracing
	attributes := map[string]string{
		"notification_id": event.NotificationID,
		"merchant_id":     event.MerchantID,
	}
	if event.RequestID != "" {
		attributes["request_id"] = event.RequestID
	}

	msg := &pubsub.Message{
		Data:       data,
		Attributes: attributes,
	}

	p.logger.Info("[GooglePubSub] Publishing event",
		slog.String("notification_id", event.NotificationID),
		slog.Int("subscriber_count", len(event.SubscriberIDs)),
	)

	// Publish message
	result := p.publisher.Publish(ctx, msg)

	// Wait for publish result
	serverID, err := result.Get(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	p.logger.Info("[GooglePubSub] Event published successfully",
		slog.String("notification_id", event.NotificationID),
		slog.String("server_id", serverID),
	)

	return nil
}

// Close releases Pub/Sub client resources
func (p *googlePubSubPublisher) Close() error {
	if p.publisher != nil {
		p.publisher.Stop()
	}
	if p.client != nil {
		return errors.WithStack(p.client.Close())
	}

	return nil
}
