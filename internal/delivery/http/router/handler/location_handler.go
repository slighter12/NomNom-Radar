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
	"go.uber.org/fx"
)

const (
	roleMerchant = "merchant"
)

// LocationHandlerParams holds dependencies for LocationHandler, injected by Fx.
type LocationHandlerParams struct {
	fx.In

	LocationUC usecase.LocationUsecase
	Logger     *slog.Logger
}

// LocationHandler holds dependencies for location-related handlers
type LocationHandler struct {
	locationUC usecase.LocationUsecase
	logger     *slog.Logger
}

// NewLocationHandler is the constructor for LocationHandler
func NewLocationHandler(params LocationHandlerParams) *LocationHandler {
	return &LocationHandler{
		locationUC: params.LocationUC,
		logger:     params.Logger,
	}
}

// CreateLocationRequest represents the request body for creating a location
type CreateLocationRequest struct {
	Label       string  `json:"label" validate:"required"`
	FullAddress string  `json:"full_address" validate:"required"`
	Latitude    float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude   float64 `json:"longitude" validate:"required,min=-180,max=180"`
	IsPrimary   bool    `json:"is_primary"`
	IsActive    bool    `json:"is_active"`
}

// UpdateLocationRequest represents the request body for updating a location
type UpdateLocationRequest struct {
	Label       *string  `json:"label,omitempty"`
	FullAddress *string  `json:"full_address,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty" validate:"omitempty,min=-90,max=90"`
	Longitude   *float64 `json:"longitude,omitempty" validate:"omitempty,min=-180,max=180"`
	IsPrimary   *bool    `json:"is_primary,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// CreateUserLocation handles creating a new user location
func (h *LocationHandler) CreateUserLocation(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	var req CreateLocationRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid location input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	input := &usecase.AddLocationInput{
		Label:       req.Label,
		FullAddress: req.FullAddress,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsPrimary:   req.IsPrimary,
		IsActive:    req.IsActive,
	}

	location, err := h.locationUC.AddUserLocation(c.Request().Context(), userID, input)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, location, "User location created successfully")
}

// GetUserLocations handles retrieving all user locations
func (h *LocationHandler) GetUserLocations(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	locations, err := h.locationUC.GetUserLocations(c.Request().Context(), userID)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, locations, "User locations retrieved successfully")
}

// UpdateUserLocation handles updating a user location
func (h *LocationHandler) UpdateUserLocation(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	locationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid location ID")
	}

	var req UpdateLocationRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid location input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	input := &usecase.UpdateLocationInput{
		Label:       req.Label,
		FullAddress: req.FullAddress,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsPrimary:   req.IsPrimary,
		IsActive:    req.IsActive,
	}

	location, err := h.locationUC.UpdateUserLocation(c.Request().Context(), userID, locationID, input)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, location, "User location updated successfully")
}

// DeleteUserLocation handles deleting a user location
func (h *LocationHandler) DeleteUserLocation(c echo.Context) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	locationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid location ID")
	}

	if err := h.locationUC.DeleteUserLocation(c.Request().Context(), userID, locationID); err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Location deleted successfully"}, "User location deleted successfully")
}

// CreateMerchantLocation handles creating a new merchant location
func (h *LocationHandler) CreateMerchantLocation(c echo.Context) error {
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	var req CreateLocationRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid location input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	input := &usecase.AddLocationInput{
		Label:       req.Label,
		FullAddress: req.FullAddress,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsPrimary:   req.IsPrimary,
		IsActive:    req.IsActive,
	}

	location, err := h.locationUC.AddMerchantLocation(c.Request().Context(), merchantID, input)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, location, "Merchant location created successfully")
}

// GetMerchantLocations handles retrieving all merchant locations
func (h *LocationHandler) GetMerchantLocations(c echo.Context) error {
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	locations, err := h.locationUC.GetMerchantLocations(c.Request().Context(), merchantID)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, locations, "Merchant locations retrieved successfully")
}

// UpdateMerchantLocation handles updating a merchant location
func (h *LocationHandler) UpdateMerchantLocation(c echo.Context) error {
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	locationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid location ID")
	}

	var req UpdateLocationRequest
	if err := c.Bind(&req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid location input")
	}

	if err := c.Validate(&req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	input := &usecase.UpdateLocationInput{
		Label:       req.Label,
		FullAddress: req.FullAddress,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsPrimary:   req.IsPrimary,
		IsActive:    req.IsActive,
	}

	location, err := h.locationUC.UpdateMerchantLocation(c.Request().Context(), merchantID, locationID, input)
	if err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, location, "Merchant location updated successfully")
}

// DeleteMerchantLocation handles deleting a merchant location
func (h *LocationHandler) DeleteMerchantLocation(c echo.Context) error {
	merchantID, err := h.getMerchantID(c)
	if err != nil {
		return err
	}

	locationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid location ID")
	}

	if err := h.locationUC.DeleteMerchantLocation(c.Request().Context(), merchantID, locationID); err != nil {
		return h.handleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Location deleted successfully"}, "Merchant location deleted successfully")
}

// getUserID extracts the user ID from the context
func (h *LocationHandler) getUserID(c echo.Context) (uuid.UUID, error) {
	userIDVal := c.Get("userID")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	return userID, nil
}

// getMerchantID extracts the merchant ID from the context
// For merchant operations, we use the userID from the token as the merchantID
func (h *LocationHandler) getMerchantID(c echo.Context) (uuid.UUID, error) {
	// Check if user has merchant role
	rolesVal := c.Get("roles")
	roles, ok := rolesVal.([]string)
	if !ok {
		return uuid.Nil, response.Forbidden(c, "FORBIDDEN", "Role information missing")
	}

	hasMerchantRole := false
	for _, role := range roles {
		if role == roleMerchant {
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
func (h *LocationHandler) handleAppError(c echo.Context, err error) error {
	var appErr domainerrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.HTTPCode(), appErr.ErrorCode(), appErr.Message(), appErr.Details())
	}

	return errors.WithStack(err)
}
