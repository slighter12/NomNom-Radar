package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/response"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

// NotificationHandlerParams holds dependencies for NotificationHandler, injected by Fx.
type NotificationHandlerParams struct {
	fx.In

	NotificationUC usecase.NotificationUsecase
	Logger         *slog.Logger
}

// NotificationHandler holds dependencies for notification-related handlers
type NotificationHandler struct {
	notificationUC usecase.NotificationUsecase
	logger         *slog.Logger
}

// NewNotificationHandler is the constructor for NotificationHandler
func NewNotificationHandler(params NotificationHandlerParams) *NotificationHandler {
	return &NotificationHandler{
		notificationUC: params.NotificationUC,
		logger:         params.Logger,
	}
}

// PublishNotificationRequest represents the request body for publishing a notification
type PublishNotificationRequest struct {
	AddressID    *uuid.UUID            `json:"address_id,omitempty"`
	LocationData *usecase.LocationData `json:"location_data,omitempty"`
	HintMessage  string                `json:"hint_message,omitempty"`
}

const (
	defaultNotificationHistoryLimit  = 20
	defaultNotificationHistoryOffset = 0
	maxNotificationHistoryLimit      = 100
)

type NotificationHistoryQueryParams struct {
	LimitOffsetQueryParams
}

// PublishLocationNotification handles publishing a location notification
func (h *NotificationHandler) PublishLocationNotification(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	var req PublishNotificationRequest
	if err := bindRequest(c, &req, "Invalid notification input"); err != nil {
		return err
	}

	// Validate request
	if err := h.validatePublishNotificationRequest(c, &req); err != nil {
		return err
	}

	notification, err := h.notificationUC.PublishLocationNotification(
		c.Request().Context(),
		merchantID,
		req.AddressID,
		req.LocationData,
		req.HintMessage,
	)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, notification)
}

// validatePublishNotificationRequest validates the publish notification request
func (h *NotificationHandler) validatePublishNotificationRequest(c echo.Context, req *PublishNotificationRequest) error {
	// Validate that either addressID or locationData is provided
	if req.AddressID == nil && req.LocationData == nil {
		return response.BadRequest(c, "VALIDATION_ERROR", "Either address_id or location_data must be provided")
	}

	// Validate that both are not provided
	if req.AddressID != nil && req.LocationData != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", "Only one of address_id or location_data should be provided")
	}

	// Validate location data if provided
	if req.LocationData != nil {
		return h.validateLocationData(c, req.LocationData)
	}

	return nil
}

// validateLocationData validates the location data
func (h *NotificationHandler) validateLocationData(c echo.Context, data *usecase.LocationData) error {
	if data.LocationName == "" {
		return response.BadRequest(c, "VALIDATION_ERROR", "location_name is required in location_data")
	}
	if data.FullAddress == "" {
		return response.BadRequest(c, "VALIDATION_ERROR", "full_address is required in location_data")
	}
	if data.Latitude < -90 || data.Latitude > 90 {
		return response.BadRequest(c, "VALIDATION_ERROR", "latitude must be between -90 and 90")
	}
	if data.Longitude < -180 || data.Longitude > 180 {
		return response.BadRequest(c, "VALIDATION_ERROR", "longitude must be between -180 and 180")
	}

	return nil
}

// GetMerchantNotificationHistory handles retrieving notification history for a merchant
func (h *NotificationHandler) GetMerchantNotificationHistory(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	query, err := h.parseNotificationHistoryQueryParams(c)
	if err != nil {
		return err
	}

	notifications, err := h.notificationUC.GetMerchantNotificationHistory(c.Request().Context(), merchantID, query.Limit, query.Offset)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, notifications)
}

func (h *NotificationHandler) parseNotificationHistoryQueryParams(c echo.Context) (NotificationHistoryQueryParams, error) {
	query := newNotificationHistoryQueryParams()

	if limitValue := strings.TrimSpace(c.QueryParam("limit")); limitValue != "" {
		limit, err := strconv.Atoi(limitValue)
		if err != nil || limit <= 0 {
			return query, response.BadRequest(c, "VALIDATION_ERROR", "limit 必須為大於 0 的整數")
		}
		query.Limit = min(limit, maxNotificationHistoryLimit)
	}

	if offsetValue := strings.TrimSpace(c.QueryParam("offset")); offsetValue != "" {
		offset, err := strconv.Atoi(offsetValue)
		if err != nil || offset < 0 {
			return query, response.BadRequest(c, "VALIDATION_ERROR", "offset 必須為大於或等於 0 的整數")
		}
		query.Offset = offset
	}

	return query, nil
}

func newNotificationHistoryQueryParams() NotificationHistoryQueryParams {
	return NotificationHistoryQueryParams{
		LimitOffsetQueryParams: NewLimitOffsetQueryParams(defaultNotificationHistoryLimit, defaultNotificationHistoryOffset),
	}
}
