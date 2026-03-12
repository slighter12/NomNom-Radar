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
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	var req CreateLocationRequest
	if err := bindAndValidateRequest(c, &req, "Invalid location input"); err != nil {
		return err
	}

	location, err := h.locationUC.AddUserLocation(c.Request().Context(), userID, newAddLocationInput(&req))
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, location)
}

// GetUserLocations handles retrieving all user locations
func (h *LocationHandler) GetUserLocations(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	locations, err := h.locationUC.GetUserLocations(c.Request().Context(), userID)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, locations)
}

// UpdateUserLocation handles updating a user location
func (h *LocationHandler) UpdateUserLocation(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	locationID, err := h.parseLocationID(c)
	if err != nil {
		return err
	}

	var req UpdateLocationRequest
	if err := bindAndValidateRequest(c, &req, "Invalid location input"); err != nil {
		return err
	}

	location, err := h.locationUC.UpdateUserLocation(c.Request().Context(), userID, locationID, newUpdateLocationInput(&req))
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, location)
}

// DeleteUserLocation handles deleting a user location
func (h *LocationHandler) DeleteUserLocation(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	locationID, err := h.parseLocationID(c)
	if err != nil {
		return err
	}

	if err := h.locationUC.DeleteUserLocation(c.Request().Context(), userID, locationID); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Location deleted successfully"})
}

// CreateMerchantLocation handles creating a new merchant location
func (h *LocationHandler) CreateMerchantLocation(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	var req CreateLocationRequest
	if err := bindAndValidateRequest(c, &req, "Invalid location input"); err != nil {
		return err
	}

	location, err := h.locationUC.AddMerchantLocation(c.Request().Context(), merchantID, newAddLocationInput(&req))
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, location)
}

// GetMerchantLocations handles retrieving all merchant locations
func (h *LocationHandler) GetMerchantLocations(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	locations, err := h.locationUC.GetMerchantLocations(c.Request().Context(), merchantID)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, locations)
}

// UpdateMerchantLocation handles updating a merchant location
func (h *LocationHandler) UpdateMerchantLocation(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	locationID, err := h.parseLocationID(c)
	if err != nil {
		return err
	}

	var req UpdateLocationRequest
	if err := bindAndValidateRequest(c, &req, "Invalid location input"); err != nil {
		return err
	}

	location, err := h.locationUC.UpdateMerchantLocation(c.Request().Context(), merchantID, locationID, newUpdateLocationInput(&req))
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, location)
}

// DeleteMerchantLocation handles deleting a merchant location
func (h *LocationHandler) DeleteMerchantLocation(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	locationID, err := h.parseLocationID(c)
	if err != nil {
		return err
	}

	if err := h.locationUC.DeleteMerchantLocation(c.Request().Context(), merchantID, locationID); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Location deleted successfully"})
}

func (h *LocationHandler) parseLocationID(c echo.Context) (uuid.UUID, error) {
	return bindLocationIDPathParam(c, "Invalid location ID")
}

func newAddLocationInput(req *CreateLocationRequest) *usecase.AddLocationInput {
	return &usecase.AddLocationInput{
		Label:       req.Label,
		FullAddress: req.FullAddress,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsPrimary:   req.IsPrimary,
		IsActive:    req.IsActive,
	}
}

func newUpdateLocationInput(req *UpdateLocationRequest) *usecase.UpdateLocationInput {
	return &usecase.UpdateLocationInput{
		Label:       req.Label,
		FullAddress: req.FullAddress,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsPrimary:   req.IsPrimary,
		IsActive:    req.IsActive,
	}
}
