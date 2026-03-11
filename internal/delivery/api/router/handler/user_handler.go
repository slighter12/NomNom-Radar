// Package handlers contains the HTTP handlers for the application.
package handler

import (
	"log/slog"
	"net/http"

	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/response"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

// UserHandlerParams holds dependencies for UserHandler, injected by Fx.
type UserHandlerParams struct {
	fx.In

	UserUC        usecase.UserUsecase
	ProfileUC     usecase.ProfileUsecase
	Logger        *slog.Logger
	GoogleAuthSVC service.OAuthAuthService
}

// UserHandler holds dependencies for user-related handlers.
type UserHandler struct {
	userUC        usecase.UserUsecase
	profileUC     usecase.ProfileUsecase
	logger        *slog.Logger
	googleAuthSVC service.OAuthAuthService
}

type GoogleCallbackQueryParams struct {
	Code  string `query:"code"`
	State string `query:"state"`
}

// NewUserHandler is the constructor for UserHandler, injected by Fx.
func NewUserHandler(params UserHandlerParams) *UserHandler {
	return &UserHandler{
		userUC:        params.UserUC,
		profileUC:     params.ProfileUC,
		logger:        params.Logger,
		googleAuthSVC: params.GoogleAuthSVC,
	}
}

// RegisterUser handles the user registration request.
func (h *UserHandler) RegisterUser(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.RegisterUserInput](c, "Invalid registration input")
	if err != nil {
		return err
	}

	output, err := h.userUC.RegisterUser(c.Request().Context(), input)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	// Do not return sensitive data in the response.
	// The DTO from the usecase might need to be mapped to a response model.
	return response.Success(c, http.StatusCreated, output.User)
}

// RegisterMerchant handles the merchant registration request.
func (h *UserHandler) RegisterMerchant(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.RegisterMerchantInput](c, "Invalid registration input")
	if err != nil {
		return err
	}

	output, err := h.userUC.RegisterMerchant(c.Request().Context(), input)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, output.User)
}

// Login handles the user login request.
func (h *UserHandler) Login(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.LoginInput](c, "Invalid login input")
	if err != nil {
		return err
	}

	output, err := h.userUC.Login(c.Request().Context(), input)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, output)
}

// RefreshToken handles the token refresh request.
func (h *UserHandler) RefreshToken(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.RefreshTokenInput](c, "Invalid refresh token input")
	if err != nil {
		return err
	}

	output, err := h.userUC.RefreshToken(c.Request().Context(), input)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, output)
}

// Logout handles the user logout request.
func (h *UserHandler) Logout(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.LogoutInput](c, "Invalid logout input")
	if err != nil {
		return err
	}

	if err := h.userUC.Logout(c.Request().Context(), input); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Successfully logged out"})
}

// GoogleCallback handles the Google Sign-In callback.
func (h *UserHandler) GoogleCallback(c echo.Context) error {
	// Extract input parameters
	input, err := h.extractGoogleCallbackInput(c)
	if err != nil {
		return err
	}

	// Process the callback
	output, err := h.userUC.GoogleCallback(c.Request().Context(), input)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, output)
}

// extractGoogleCallbackInput extracts and validates input from the request
func (h *UserHandler) extractGoogleCallbackInput(c echo.Context) (*usecase.GoogleCallbackInput, error) {
	var query GoogleCallbackQueryParams
	if err := bindQueryParams(c, &query); err != nil {
		return nil, response.BindingError(c, "INVALID_INPUT", "Invalid Google callback input")
	}

	code := query.Code
	idToken := c.FormValue("id_token")
	state := query.State

	// Handle authorization code flow (not implemented yet)
	if code != "" {
		return nil, response.BadRequest(c, "INVALID_INPUT", "Authorization code flow not implemented yet. Please use ID token flow.")
	}

	// Handle ID token flow
	if idToken != "" {
		return &usecase.GoogleCallbackInput{
			IDToken: idToken,
			State:   state,
		}, nil
	}

	// Handle JSON body binding
	input, err := bindRequiredPayload[usecase.GoogleCallbackInput](c, "Invalid Google callback input")
	if err != nil {
		return nil, err
	}

	// Override state if provided in query params
	if state != "" {
		input.State = state
	}

	// Validate required fields
	if input.IDToken == "" {
		return nil, response.BadRequest(c, "INVALID_INPUT", "ID token is required")
	}

	return input, nil
}

// GetProfile handles the request to get the current user's profile.
func (h *UserHandler) GetProfile(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	user, err := h.profileUC.GetProfile(c.Request().Context(), userID)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, user)
}

// HealthCheck is a simple handler to check if the service is up.
func HealthCheck(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{"status": "ok"})
}
