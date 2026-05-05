package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/response"
	"radar/internal/domain/entity"
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
	input.Email = entity.NormalizeEmail(input.Email)

	output, err := h.userUC.RegisterUser(c.Request().Context(), input)
	if err != nil {
		return withSourceStack(err)
	}

	// Do not return sensitive data in the response.
	// The DTO from the usecase might need to be mapped to a response model.
	return response.Success(c, http.StatusCreated, output)
}

// RegisterMerchant handles the merchant registration request.
func (h *UserHandler) RegisterMerchant(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.RegisterMerchantInput](c, "Invalid registration input")
	if err != nil {
		return err
	}
	input.Email = entity.NormalizeEmail(input.Email)

	output, err := h.userUC.RegisterMerchant(c.Request().Context(), input)
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusCreated, output)
}

// Login handles the user login request.
func (h *UserHandler) Login(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.LoginInput](c, "Invalid login input")
	if err != nil {
		return err
	}
	input.Email = entity.NormalizeEmail(input.Email)

	output, err := h.userUC.Login(c.Request().Context(), input)
	if err != nil {
		setRetryAfterHeaderOnLockout(c, err)

		return withSourceStack(err)
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
		return withSourceStack(err)
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
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, map[string]string{responseKeyMessage: "Successfully logged out"})
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
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, output)
}

// CompleteMerchantOnboarding finalizes merchant onboarding for a verified identity.
func (h *UserHandler) CompleteMerchantOnboarding(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.CompleteMerchantOnboardingInput](c, "Invalid merchant onboarding input")
	if err != nil {
		return err
	}

	output, err := h.userUC.CompleteMerchantOnboarding(c.Request().Context(), input)
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, output)
}

func (h *UserHandler) SubmitMerchantVerification(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	input, err := bindRequiredPayload[usecase.SubmitMerchantVerificationInput](c, "Invalid merchant verification input")
	if err != nil {
		return err
	}

	if err := h.profileUC.SubmitMerchantVerification(c.Request().Context(), userID, input); err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, map[string]string{responseKeyStatus: "verified"})
}

func (h *UserHandler) LinkProvider(c echo.Context) error {
	input, err := bindRequiredPayload[usecase.LinkProviderInput](c, "Invalid link provider input")
	if err != nil {
		return err
	}

	output, err := h.userUC.LinkProvider(c.Request().Context(), *input)
	if err != nil {
		setRetryAfterHeaderOnLockout(c, err)

		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, output)
}

// extractGoogleCallbackInput extracts and validates input from the request.
// Query params (code, state) are read from the URL; the rest comes from the JSON body.
func (h *UserHandler) extractGoogleCallbackInput(c echo.Context) (*usecase.GoogleCallbackInput, error) {
	var query GoogleCallbackQueryParams
	if err := bindQueryParams(c, &query); err != nil {
		return nil, response.BindingError(c, "INVALID_INPUT", "Invalid Google callback input")
	}

	// Handle authorization code flow (not implemented yet)
	if query.Code != "" {
		return nil, response.BadRequest(c, "INVALID_INPUT", "Authorization code flow not implemented yet. Please use ID token flow.")
	}

	input, err := bindRequiredPayload[usecase.GoogleCallbackInput](c, "Invalid Google callback input")
	if err != nil {
		return nil, err
	}

	// Backward compatibility: query param state overrides body
	if query.State != "" {
		input.State = query.State
	}

	if err := validateRequest(c, input); err != nil {
		return nil, err
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
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, user)
}

func setRetryAfterHeaderOnLockout(c echo.Context, err error) {
	if lockoutErr, ok := errors.AsType[*usecase.LockoutError](err); ok {
		c.Response().Header().Set("Retry-After", strconv.Itoa(lockoutErr.RetryAfterSeconds))
	}
}

// HealthCheck is a simple handler to check if the service is up.
func HealthCheck(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{responseKeyStatus: "ok"})
}
