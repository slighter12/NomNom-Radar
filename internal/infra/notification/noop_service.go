package notification

import (
	"context"

	"radar/internal/domain/service"
)

type noopService struct{}

func NewNoopNotificationService() service.NotificationService {
	return &noopService{}
}

func (s *noopService) SendBatchNotification(
	_ context.Context,
	tokens []string,
	_, _ string,
	_ map[string]string,
) (successCount, failureCount int, invalidTokens []string, err error) {
	return len(tokens), 0, nil, nil
}

func (s *noopService) SendSingleNotification(
	_ context.Context,
	_, _, _ string,
	_ map[string]string,
) error {
	return nil
}
