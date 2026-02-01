package handler

import (
	"log/slog"
	"net/http"

	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/response"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

// DeviceHandlerParams holds dependencies for DeviceHandler, injected by Fx.
type DeviceHandlerParams struct {
	fx.In

	DeviceUC usecase.DeviceUsecase
	Logger   *slog.Logger
}

// DeviceHandler holds dependencies for device-related handlers
type DeviceHandler struct {
	deviceUC usecase.DeviceUsecase
	logger   *slog.Logger
}

// NewDeviceHandler is the constructor for DeviceHandler
func NewDeviceHandler(params DeviceHandlerParams) *DeviceHandler {
	return &DeviceHandler{
		deviceUC: params.DeviceUC,
		logger:   params.Logger,
	}
}

// RegisterDeviceRequest represents the request body for registering a device
type RegisterDeviceRequest struct {
	FCMToken string `json:"fcm_token" validate:"required"`
	DeviceID string `json:"device_id" validate:"required"`
	Platform string `json:"platform" validate:"required,oneof=ios android"`
}

// UpdateFCMTokenRequest represents the request body for updating FCM token
type UpdateFCMTokenRequest struct {
	FCMToken string `json:"fcm_token" validate:"required"`
}

// RegisterDevice handles device registration
func (h *DeviceHandler) RegisterDevice(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	var req RegisterDeviceRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid device input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	deviceInfo := &usecase.DeviceInfo{
		FCMToken: req.FCMToken,
		DeviceID: req.DeviceID,
		Platform: req.Platform,
	}

	device, err := h.deviceUC.RegisterDevice(c.Request().Context(), userID, deviceInfo)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, device)
}

// GetUserDevices handles retrieving all user devices
func (h *DeviceHandler) GetUserDevices(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	devices, err := h.deviceUC.GetUserDevices(c.Request().Context(), userID)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, devices)
}

// UpdateFCMToken handles updating FCM token for a device
func (h *DeviceHandler) UpdateFCMToken(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid device ID")
	}

	var req UpdateFCMTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid FCM token input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	if err := h.deviceUC.UpdateFCMToken(c.Request().Context(), userID, deviceID, req.FCMToken); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "FCM token updated successfully"})
}

// DeactivateDevice handles deactivating a device
func (h *DeviceHandler) DeactivateDevice(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid device ID")
	}

	if err := h.deviceUC.DeactivateDevice(c.Request().Context(), userID, deviceID); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Device deactivated successfully"})
}
