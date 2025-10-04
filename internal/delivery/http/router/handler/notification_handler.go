package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"radar/internal/delivery/http/response"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// NotificationHandler holds dependencies for notification-related handlers
type NotificationHandler struct {
	uc     usecase.NotificationUsecase
	logger *slog.Logger
}

// NewNotificationHandler is the constructor for NotificationHandler
func NewNotificationHandler(uc usecase.NotificationUsecase, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{
		uc:     uc,
		logger: logger,
	}
}

// PublishNotificationRequest represents the request body for publishing a notification
type PublishNotificationRequest struct {
	AddressID    *uuid.UUID            `json:"address_id,omitempty"`
	LocationData *usecase.LocationData `json:"location_data,omitempty"`
	HintMessage  string                `json:"hint_message,omitempty"`
}

// PublishLocationNotification handles publishing a location notification
func (h *NotificationHandler) PublishLocationNotification(c echo.Context) error {
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	var req PublishNotificationRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid notification input")
	}

	// Validate request
	if err := h.validatePublishNotificationRequest(c, &req); err != nil {
		return err
	}

	notification, err := h.uc.PublishLocationNotification(
		c.Request().Context(),
		merchantID,
		req.AddressID,
		req.LocationData,
		req.HintMessage,
	)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, notification, "Location notification published successfully")
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
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	// Parse pagination parameters
	limit := 20 // default limit
	offset := 0 // default offset

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	notifications, err := h.uc.GetMerchantNotificationHistory(c.Request().Context(), merchantID, limit, offset)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, notifications, "Notification history retrieved successfully")
}

// getMerchantID extracts the merchant ID from the context and verifies merchant role
func (h *NotificationHandler) getMerchantID(c echo.Context) (uuid.UUID, error) {
	// Check if user has merchant role
	rolesVal := c.Get("roles")
	roles, ok := rolesVal.([]string)
	if !ok {
		return uuid.Nil, response.Forbidden(c, "FORBIDDEN", "Role information missing")
	}

	hasMerchantRole := false
	for _, role := range roles {
		if role == "merchant" {
			hasMerchantRole = true

			break
		}
	}

	if !hasMerchantRole {
		return uuid.Nil, response.Forbidden(c, "FORBIDDEN", "Merchant role required")
	}

	// Get the user ID which is the merchant ID
	userIDVal := c.Get("userID")
	merchantID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, response.Unauthorized(c, "INVALID_TOKEN", "Invalid merchant ID in token")
	}

	return merchantID, nil
}

// handleAppError handles application errors
func (h *NotificationHandler) handleAppError(c echo.Context, err error) error {
	var appErr domainerrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.HTTPCode(), appErr.ErrorCode(), appErr.Message(), appErr.Details())
	}

	return errors.WithStack(err)
}
