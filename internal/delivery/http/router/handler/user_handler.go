// Package handlers contains the HTTP handlers for the application.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"radar/internal/usecase"
)

// UserHandler holds dependencies for user-related handlers.
type UserHandler struct {
	uc     usecase.UserUsecase
	logger *slog.Logger
}

// NewUserHandler is the constructor for UserHandler, injected by Fx.
func NewUserHandler(uc usecase.UserUsecase, logger *slog.Logger) *UserHandler {
	return &UserHandler{uc: uc, logger: logger}
}

// RegisterUser handles the user registration request.
func (h *UserHandler) RegisterUser(c echo.Context) error {
	var input usecase.RegisterUserInput
	if err := c.Bind(&input); err != nil {
		h.logger.Warn("Failed to bind registration input", "error", err.Error())
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	output, err := h.uc.RegisterUser(c.Request().Context(), input)
	if err != nil {
		// Here you can map domain errors to specific HTTP status codes
		h.logger.Error("User registration failed", "error", err.Error())
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	}

	// Do not return sensitive data in the response.
	// The DTO from the usecase might need to be mapped to a response model.
	return c.JSON(http.StatusCreated, output.User)
}

// Login handles the user login request.
func (h *UserHandler) Login(c echo.Context) error {
	var input usecase.LoginInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	output, err := h.uc.Login(c.Request().Context(), input)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, output)
}

// GetProfile handles the request to get the current user's profile.
func (h *UserHandler) GetProfile(c echo.Context) error {
	userIDVal := c.Get("userID")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid user ID in token"})
	}

	// In a real app, you would have a GetProfile use case
	// user, err := h.uc.GetProfile(c.Request().Context(), userID)

	// For now, just return the ID
	return c.JSON(http.StatusOK, map[string]string{"message": "Welcome!", "user_id": userID.String()})
}

// HealthCheck is a simple handler to check if the service is up.
func HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
