package pubsub

import (
	"context"
	"log/slog"

	"radar/config"
	"radar/internal/domain/constants"
	"radar/internal/domain/service"

	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// noopPublisher is a no-op implementation when Pub/Sub is disabled
type noopPublisher struct {
	logger *slog.Logger
}

func (p *noopPublisher) PublishNotificationEvent(ctx context.Context, event *service.NotificationEvent) error {
	p.logger.Debug("[NoopPubSub] Event publishing disabled, skipping",
		slog.String("notification_id", event.NotificationID),
	)

	return nil
}

func (p *noopPublisher) Close() error {
	return nil
}

// PublisherParams holds dependencies for EventPublisher, injected by Fx
type PublisherParams struct {
	fx.In

	Lc     fx.Lifecycle
	Ctx    context.Context
	Config *config.Config
	Logger *slog.Logger
}

// NewEventPublisher creates an EventPublisher based on configuration
func NewEventPublisher(params PublisherParams) (service.EventPublisher, error) {
	cfg := params.Config.PubSub
	logger := params.Logger

	// If PubSub is not configured, return a no-op publisher
	if cfg == nil || cfg.Provider == "" {
		logger.Info("PubSub not configured, using no-op publisher")

		return &noopPublisher{logger: logger}, nil
	}

	var publisher service.EventPublisher
	var err error

	switch cfg.Provider {
	case constants.PubSubProviderLocal:
		if cfg.LocalEndpoint == "" {
			return nil, errors.New("local endpoint is required for local provider")
		}
		logger.Info("Using local HTTP publisher for Pub/Sub",
			slog.String("endpoint", cfg.LocalEndpoint),
		)

		publisher = NewLocalHTTPPublisher(cfg.LocalEndpoint, logger)

	case constants.PubSubProviderGoogle:
		if cfg.ProjectID == "" {
			return nil, errors.New("project ID is required for google provider")
		}
		if cfg.TopicID == "" {
			return nil, errors.New("topic ID is required for google provider")
		}
		logger.Info("Using Google Pub/Sub publisher",
			slog.String("project_id", cfg.ProjectID),
			slog.String("topic_id", cfg.TopicID),
		)

		publisher, err = NewGooglePubSubPublisher(params.Ctx, cfg.ProjectID, cfg.TopicID, logger)
		if err != nil {
			return nil, err
		}

	default:
		return nil, errors.Errorf("unknown pubsub provider: %s", cfg.Provider)
	}

	// Register lifecycle hook to close publisher on shutdown
	params.Lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing EventPublisher")

			return publisher.Close()
		},
	})

	return publisher, nil
}

// Module provides the Pub/Sub FX module
//
//nolint:gochecknoglobals
var Module = fx.Options(
	fx.Provide(NewEventPublisher),
)
