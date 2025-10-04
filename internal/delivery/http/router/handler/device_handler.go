package handler

import (
	"log/slog"
	"net/http"

	"radar/internal/delivery/http/response"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// DeviceHandler holds dependencies for device-related handlers
type DeviceHandler struct {
	uc     usecase.DeviceUsecase
	logger *slog.Logger
}

// NewDeviceHandler is the constructor for DeviceHandler
func NewDeviceHandler(uc usecase.DeviceUsecase, logger *slog.Logger) *DeviceHandler {
	return &DeviceHandler{
		uc:     uc,
		logger: logger,
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
	userID, err := h.getUserID(c)
	if err != nil {
		return err
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

	device, err := h.uc.RegisterDevice(c.Request().Context(), userID, deviceInfo)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, device, "Device registered successfully")
}

// GetUserDevices handles retrieving all user devices
func (h *DeviceHandler) GetUserDevices(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	devices, err := h.uc.GetUserDevices(c.Request().Context(), userID)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, devices, "User devices retrieved successfully")
}

// UpdateFCMToken handles updating FCM token for a device
func (h *DeviceHandler) UpdateFCMToken(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
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

	if err := h.uc.UpdateFCMToken(c.Request().Context(), userID, deviceID, req.FCMToken); err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "FCM token updated successfully"}, "FCM token updated successfully")
}

// DeactivateDevice handles deactivating a device
func (h *DeviceHandler) DeactivateDevice(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid device ID")
	}

	if err := h.uc.DeactivateDevice(c.Request().Context(), userID, deviceID); err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Device deactivated successfully"}, "Device deactivated successfully")
}

// getUserID extracts the user ID from the context
func (h *DeviceHandler) getUserID(c echo.Context) (uuid.UUID, error) {
	userIDVal := c.Get("userID")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	return userID, nil
}

// handleAppError handles application errors
func (h *DeviceHandler) handleAppError(c echo.Context, err error) error {
	var appErr domainerrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.HTTPCode(), appErr.ErrorCode(), appErr.Message(), appErr.Details())
	}

	return errors.WithStack(err)
}
