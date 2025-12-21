package handler

import (
	"log/slog"
	"net/http"

	"radar/internal/delivery/http/response"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

// SubscriptionHandlerParams holds dependencies for SubscriptionHandler, injected by Fx.
type SubscriptionHandlerParams struct {
	fx.In

	SubscriptionUC usecase.SubscriptionUsecase
	Logger         *slog.Logger
}

// SubscriptionHandler holds dependencies for subscription-related handlers
type SubscriptionHandler struct {
	subscriptionUC usecase.SubscriptionUsecase
	logger         *slog.Logger
}

// NewSubscriptionHandler is the constructor for SubscriptionHandler
func NewSubscriptionHandler(params SubscriptionHandlerParams) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptionUC: params.SubscriptionUC,
		logger:         params.Logger,
	}
}

// SubscribeRequest represents the request body for subscribing to a merchant
type SubscribeRequest struct {
	MerchantID uuid.UUID           `json:"merchant_id" validate:"required"`
	DeviceInfo *usecase.DeviceInfo `json:"device_info,omitempty"`
}

// ProcessQRRequest represents the request body for processing QR subscription
type ProcessQRRequest struct {
	QRData     string              `json:"qr_data" validate:"required"`
	DeviceInfo *usecase.DeviceInfo `json:"device_info,omitempty"`
}

// SubscribeToMerchant handles subscribing to a merchant
func (h *SubscriptionHandler) SubscribeToMerchant(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	var req SubscribeRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid subscription input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	subscription, err := h.subscriptionUC.SubscribeToMerchant(c.Request().Context(), userID, req.MerchantID, req.DeviceInfo)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, subscription, "Subscribed to merchant successfully")
}

// UnsubscribeFromMerchant handles unsubscribing from a merchant
func (h *SubscriptionHandler) UnsubscribeFromMerchant(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid merchant ID")
	}

	if err := h.subscriptionUC.UnsubscribeFromMerchant(c.Request().Context(), userID, merchantID); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Unsubscribed successfully"}, "Unsubscribed from merchant successfully")
}

// GetUserSubscriptions handles retrieving all user subscriptions
func (h *SubscriptionHandler) GetUserSubscriptions(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	subscriptions, err := h.subscriptionUC.GetUserSubscriptions(c.Request().Context(), userID)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, subscriptions, "User subscriptions retrieved successfully")
}

// GenerateSubscriptionQR handles generating QR code for merchant subscription
func (h *SubscriptionHandler) GenerateSubscriptionQR(c echo.Context) error {
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	qrCode, err := h.subscriptionUC.GenerateSubscriptionQR(c.Request().Context(), merchantID)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	// Return QR code as PNG image
	c.Response().Header().Set("Content-Type", "image/png")
	c.Response().Header().Set("Content-Disposition", "inline; filename=subscription-qr.png")

	return c.Blob(http.StatusOK, "image/png", qrCode)
}

// ProcessQRSubscription handles processing QR code subscription
func (h *SubscriptionHandler) ProcessQRSubscription(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	var req ProcessQRRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid QR subscription input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	subscription, err := h.subscriptionUC.ProcessQRSubscription(c.Request().Context(), userID, req.QRData, req.DeviceInfo)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, subscription, "Subscribed via QR code successfully")
}

// getUserID extracts the user ID from the context
func (h *SubscriptionHandler) getUserID(c echo.Context) (uuid.UUID, error) {
	userIDVal := c.Get("userID")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	return userID, nil
}

// getMerchantID extracts the merchant ID from the context and verifies merchant role
func (h *SubscriptionHandler) getMerchantID(c echo.Context) (uuid.UUID, error) {
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
