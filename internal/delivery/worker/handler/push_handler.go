package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"radar/config"
	deliverycontext "radar/internal/delivery/context"
	"radar/internal/domain/constants"
	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/fx"
	"google.golang.org/api/idtoken"
)

// PubSubMessage represents the structure of a Pub/Sub push message
type PubSubMessage struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes,omitempty"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// retryableError wraps an error to indicate it should trigger a Pub/Sub retry
type retryableError struct {
	err error
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("retryable: %v", e.err)
}

func (e *retryableError) Unwrap() error {
	return e.err
}

// newRetryableError wraps an error as retryable
func newRetryableError(err error) error {
	return &retryableError{err: err}
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	var re *retryableError

	return errors.As(err, &re)
}

// PushHandler handles Pub/Sub push messages for geo processing
type PushHandler struct {
	verifyPushAuth   bool
	logger           *slog.Logger
	routingSvc       usecase.RoutingUsecase
	notificationSvc  service.NotificationService
	subscriptionRepo repository.SubscriptionRepository
	deviceRepo       repository.DeviceRepository
	notificationRepo repository.NotificationRepository
}

// PushHandlerParams holds dependencies for the PushHandler
type PushHandlerParams struct {
	fx.In

	Config           *config.Config
	Logger           *slog.Logger
	RoutingSvc       usecase.RoutingUsecase
	NotificationSvc  service.NotificationService
	SubscriptionRepo repository.SubscriptionRepository
	DeviceRepo       repository.DeviceRepository
	NotificationRepo repository.NotificationRepository
}

// NewPushHandler creates a new Pub/Sub push handler
func NewPushHandler(params PushHandlerParams) *PushHandler {
	// Determine if we need to verify push auth based on config
	verifyPushAuth := params.Config.PubSub != nil &&
		params.Config.PubSub.Provider == constants.PubSubProviderGoogle &&
		params.Config.Env.Env != constants.EnvDevelop

	return &PushHandler{
		verifyPushAuth:   verifyPushAuth,
		logger:           params.Logger,
		routingSvc:       params.RoutingSvc,
		notificationSvc:  params.NotificationSvc,
		subscriptionRepo: params.SubscriptionRepo,
		deviceRepo:       params.DeviceRepo,
		notificationRepo: params.NotificationRepo,
	}
}

// HandlePush handles incoming Pub/Sub push messages
func (h *PushHandler) HandlePush(c echo.Context) error {
	ctx := c.Request().Context()

	// Verify Pub/Sub token in production for Google provider
	if h.verifyPushAuth {
		if err := verifyPubSubToken(c.Request()); err != nil {
			h.logger.Warn("[Worker] Invalid Pub/Sub token", slog.Any("error", err))

			return c.NoContent(http.StatusUnauthorized)
		}
	}

	// Parse Pub/Sub message
	var pushMsg PubSubMessage
	if err := c.Bind(&pushMsg); err != nil {
		h.logger.Error("[Worker] Failed to parse push message", slog.Any("error", err))

		return c.NoContent(http.StatusBadRequest)
	}

	// Decode base64 message data
	data, err := base64.StdEncoding.DecodeString(pushMsg.Message.Data)
	if err != nil {
		h.logger.Error("[Worker] Failed to decode message data", slog.Any("error", err))

		return c.NoContent(http.StatusBadRequest)
	}

	// Parse notification event
	var event service.NotificationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		h.logger.Error("[Worker] Failed to parse notification event", slog.Any("error", err))

		return c.NoContent(http.StatusBadRequest)
	}

	// Extract request_id for distributed tracing
	// Priority: message attributes > event field > existing context
	requestID := h.extractRequestID(ctx, &pushMsg, &event)

	// Create request-scoped logger with request_id
	reqLogger := h.logger.With(slog.String("request_id", requestID))

	// Update context with request_id and logger
	ctx = deliverycontext.WithRequestID(ctx, requestID)
	ctx = deliverycontext.WithLogger(ctx, reqLogger)

	reqLogger.Info("[Worker] Processing notification event",
		slog.String("notification_id", event.NotificationID),
		slog.String("merchant_id", event.MerchantID),
		slog.Int("subscriber_count", len(event.SubscriberIDs)),
	)

	// Process the notification
	if err := h.processNotification(ctx, &event); err != nil {
		reqLogger.Error("[Worker] Failed to process notification",
			slog.String("notification_id", event.NotificationID),
			slog.Any("error", err),
			slog.Bool("retryable", isRetryableError(err)),
		)
		// Return 503 for retryable errors to trigger Pub/Sub retry
		// Return 200 for non-retryable errors to prevent infinite retries
		if isRetryableError(err) {
			return c.NoContent(http.StatusServiceUnavailable)
		}

		return c.NoContent(http.StatusOK)
	}

	reqLogger.Info("[Worker] Notification processed successfully",
		slog.String("notification_id", event.NotificationID),
	)

	return c.NoContent(http.StatusOK)
}

// extractRequestID extracts request_id from message attributes, event, or generates a new one
func (h *PushHandler) extractRequestID(ctx context.Context, pushMsg *PubSubMessage, event *service.NotificationEvent) string {
	// 1. Try message attributes (from Pub/Sub)
	if requestID, ok := pushMsg.Message.Attributes["request_id"]; ok && requestID != "" {
		return requestID
	}

	// 2. Try event field (from JSON payload)
	if event.RequestID != "" {
		return event.RequestID
	}

	// 3. Try existing context (from RequestIDMiddleware via X-Request-Id header)
	if requestID := deliverycontext.GetRequestIDFromContext(ctx); requestID != "" {
		return requestID
	}

	// 4. Generate new UUID as fallback
	return uuid.New().String()
}

// processNotification processes a notification event
func (h *PushHandler) processNotification(ctx context.Context, event *service.NotificationEvent) error {
	// Parse IDs
	notificationID, merchantID, subscriberIDs, err := h.parseEventIDs(event)
	if err != nil {
		return err
	}

	if len(subscriberIDs) == 0 {
		h.logger.Info("[Worker] No subscribers to notify",
			slog.String("notification_id", event.NotificationID),
		)

		return nil
	}

	// Filter subscribers by distance
	validUserIDs, err := h.filterSubscribersByDistance(ctx, merchantID, subscriberIDs, event)
	if err != nil {
		return err
	}

	if len(validUserIDs) == 0 {
		return nil
	}

	// Get devices for valid users
	devices, deviceMap, err := h.getDevicesForUsers(ctx, validUserIDs, event.NotificationID)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		return nil
	}

	// Prepare and send notifications
	title, body, notificationData := h.prepareNotificationContent(event)
	tokens := h.collectTokens(devices)

	totalSent, totalFailed, invalidTokens, notificationLogs := h.sendBatchedNotifications(
		ctx, tokens, deviceMap, title, body, notificationData, notificationID,
	)

	// Cleanup invalid tokens
	h.cleanupInvalidTokens(ctx, invalidTokens, deviceMap)

	// Save results
	h.saveNotificationResults(ctx, notificationID, notificationLogs, totalSent, totalFailed, len(invalidTokens), event.NotificationID)

	return nil
}

// parseEventIDs parses and validates all IDs from the event
func (h *PushHandler) parseEventIDs(event *service.NotificationEvent) (notificationID, merchantID uuid.UUID, subscriberIDs []uuid.UUID, err error) {
	notificationID, err = uuid.Parse(event.NotificationID)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, errors.WithStack(err)
	}

	merchantID, err = uuid.Parse(event.MerchantID)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, errors.WithStack(err)
	}

	subscriberIDs = make([]uuid.UUID, 0, len(event.SubscriberIDs))
	for _, idStr := range event.SubscriberIDs {
		id, parseErr := uuid.Parse(idStr)
		if parseErr == nil {
			subscriberIDs = append(subscriberIDs, id)
		}
	}

	return notificationID, merchantID, subscriberIDs, nil
}

// filterSubscribersByDistance filters subscribers based on road network distance
func (h *PushHandler) filterSubscribersByDistance(ctx context.Context, merchantID uuid.UUID, subscriberIDs []uuid.UUID, event *service.NotificationEvent) ([]uuid.UUID, error) {
	addresses, err := h.subscriptionRepo.FindSubscriberAddressesByUserIDs(ctx, merchantID, subscriberIDs)
	if err != nil {
		return nil, newRetryableError(errors.WithStack(err))
	}

	if len(addresses) == 0 {
		h.logger.Info("[Worker] No addresses found for subscribers",
			slog.String("notification_id", event.NotificationID),
		)

		return nil, nil
	}

	source := usecase.Coordinate{Lat: event.Latitude, Lng: event.Longitude}
	targets := make([]usecase.Coordinate, len(addresses))
	for idx, addr := range addresses {
		targets[idx] = usecase.Coordinate{Lat: addr.Latitude, Lng: addr.Longitude}
	}

	routeResults, err := h.routingSvc.OneToMany(ctx, source, targets)
	if err != nil {
		return nil, newRetryableError(errors.WithStack(err))
	}

	validUserIDs := make([]uuid.UUID, 0)
	for idx, result := range routeResults.Results {
		distanceMeters := result.DistanceKm * 1000.0
		if result.IsReachable && distanceMeters <= addresses[idx].NotificationRadius {
			validUserIDs = append(validUserIDs, addresses[idx].OwnerID)
		}
	}

	h.logger.Info("[Worker] Filtered subscribers by road distance",
		slog.String("notification_id", event.NotificationID),
		slog.Int("original_count", len(addresses)),
		slog.Int("valid_count", len(validUserIDs)),
	)

	return validUserIDs, nil
}

// getDevicesForUsers retrieves devices for the given user IDs
func (h *PushHandler) getDevicesForUsers(ctx context.Context, userIDs []uuid.UUID, notificationID string) ([]*entity.UserDevice, map[string]*entity.UserDevice, error) {
	devices, err := h.subscriptionRepo.FindDevicesForUsers(ctx, userIDs)
	if err != nil {
		return nil, nil, newRetryableError(errors.WithStack(err))
	}

	if len(devices) == 0 {
		h.logger.Info("[Worker] No devices found for valid subscribers",
			slog.String("notification_id", notificationID),
		)

		return nil, nil, nil
	}

	deviceMap := make(map[string]*entity.UserDevice, len(devices))
	for _, device := range devices {
		deviceMap[device.FCMToken] = device
	}

	return devices, deviceMap, nil
}

// collectTokens extracts FCM tokens from devices
func (h *PushHandler) collectTokens(devices []*entity.UserDevice) []string {
	tokens := make([]string, 0, len(devices))
	for _, device := range devices {
		tokens = append(tokens, device.FCMToken)
	}

	return tokens
}

// prepareNotificationContent creates the notification title, body, and data
func (h *PushHandler) prepareNotificationContent(event *service.NotificationEvent) (title, body string, data map[string]string) {
	title = "商戶位置通知"
	body = fmt.Sprintf("%s 已在 %s 開始營業", event.LocationName, event.FullAddress)
	if event.HintMessage != "" {
		body = fmt.Sprintf("%s - %s", body, event.HintMessage)
	}

	data = map[string]string{
		"notification_id": event.NotificationID,
		"merchant_id":     event.MerchantID,
		"latitude":        fmt.Sprintf("%f", event.Latitude),
		"longitude":       fmt.Sprintf("%f", event.Longitude),
		"location_name":   event.LocationName,
		"full_address":    event.FullAddress,
	}

	return title, body, data
}

// sendBatchedNotifications sends notifications in batches and collects results
func (h *PushHandler) sendBatchedNotifications(ctx context.Context, tokens []string, deviceMap map[string]*entity.UserDevice, title, body string, data map[string]string, notificationID uuid.UUID) (sent, failed int, invalidTokens []string, logs []*entity.NotificationLog) {
	const batchSize = 500

	totalSent := 0
	totalFailed := 0
	var allInvalidTokens []string
	var notificationLogs []*entity.NotificationLog

	for idx := 0; idx < len(tokens); idx += batchSize {
		end := min(idx+batchSize, len(tokens))
		batch := tokens[idx:end]

		successCount, failureCount, batchInvalidTokens, sendErr := h.notificationSvc.SendBatchNotification(
			ctx, batch, title, body, data,
		)

		if sendErr != nil {
			h.logger.Error("[Worker] Failed to send batch",
				slog.Int("batch_start", idx),
				slog.Int("batch_size", len(batch)),
				slog.Any("error", sendErr),
			)
			totalFailed += len(batch)

			// Create failure logs for all tokens in this batch
			for _, token := range batch {
				device, ok := deviceMap[token]
				if !ok || device == nil {
					h.logger.Warn("[Worker] Device not found for token during batch failure",
						slog.String("token_prefix", token[:min(10, len(token))]),
					)

					continue
				}

				notificationLogs = append(notificationLogs, &entity.NotificationLog{
					ID:             uuid.New(),
					NotificationID: notificationID,
					UserID:         device.UserID,
					DeviceID:       device.ID,
					Status:         "failed",
					ErrorMessage:   fmt.Sprintf("batch send error: %v", sendErr),
					SentAt:         time.Now(),
				})
			}

			continue
		}

		totalSent += successCount
		totalFailed += failureCount
		allInvalidTokens = append(allInvalidTokens, batchInvalidTokens...)

		// Create logs for this batch
		for _, token := range batch {
			device, ok := deviceMap[token]
			if !ok || device == nil {
				h.logger.Warn("[Worker] Device not found for token",
					slog.String("token_prefix", token[:min(10, len(token))]),
				)

				continue
			}

			status := "sent"
			errorMsg := ""
			if slices.Contains(batchInvalidTokens, token) {
				status = "failed"
				errorMsg = "invalid or unregistered token"
			}

			notificationLogs = append(notificationLogs, &entity.NotificationLog{
				ID:             uuid.New(),
				NotificationID: notificationID,
				UserID:         device.UserID,
				DeviceID:       device.ID,
				Status:         status,
				ErrorMessage:   errorMsg,
				SentAt:         time.Now(),
			})
		}
	}

	return totalSent, totalFailed, allInvalidTokens, notificationLogs
}

// cleanupInvalidTokens removes devices with invalid FCM tokens
func (h *PushHandler) cleanupInvalidTokens(ctx context.Context, invalidTokens []string, deviceMap map[string]*entity.UserDevice) {
	for _, token := range invalidTokens {
		if device, ok := deviceMap[token]; ok {
			if err := h.deviceRepo.DeleteDevice(ctx, device.ID); err != nil {
				h.logger.Warn("[Worker] Failed to delete invalid device",
					slog.String("device_id", device.ID.String()),
					slog.Any("error", err),
				)
			}
		}
	}
}

// saveNotificationResults saves notification logs and updates status
func (h *PushHandler) saveNotificationResults(ctx context.Context, notificationID uuid.UUID, logs []*entity.NotificationLog, sent, failed, invalidTokensCount int, eventID string) {
	if len(logs) > 0 {
		if err := h.notificationRepo.BatchCreateNotificationLogs(ctx, logs); err != nil {
			h.logger.Error("[Worker] Failed to create notification logs", slog.Any("error", err))
		}
	}

	if err := h.notificationRepo.UpdateNotificationStatus(ctx, notificationID, sent, failed); err != nil {
		h.logger.Error("[Worker] Failed to update notification status", slog.Any("error", err))
	}

	h.logger.Info("[Worker] Notification sending completed",
		slog.String("notification_id", eventID),
		slog.Int("total_sent", sent),
		slog.Int("total_failed", failed),
		slog.Int("invalid_tokens", invalidTokensCount),
	)
}

// verifyPubSubToken verifies the JWT token from Google Pub/Sub push requests
// Reference: https://cloud.google.com/pubsub/docs/push#authenticating_standard_push_requests
func verifyPubSubToken(req *http.Request) error {
	// Get the Authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return errors.New("missing authorization header")
	}

	// Extract Bearer token
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return errors.New("invalid authorization header format")
	}
	token := strings.TrimPrefix(authHeader, bearerPrefix)

	// Construct the expected audience (the push endpoint URL)
	// The audience should be the URL of this endpoint
	scheme := "https"
	if req.TLS == nil {
		scheme = "http" // For local development
	}
	audience := fmt.Sprintf("%s://%s%s", scheme, req.Host, req.URL.Path)

	// Validate the token using Google's ID token validator
	ctx := req.Context()
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return errors.Wrap(err, "failed to validate token")
	}

	// Verify the token is from Google Pub/Sub
	// The issuer should be accounts.google.com
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return errors.Errorf("invalid issuer: %s", payload.Issuer)
	}

	// Verify email is verified (if email claim exists)
	if emailVerified, ok := payload.Claims["email_verified"].(bool); ok && !emailVerified {
		return errors.New("email not verified")
	}

	return nil
}
