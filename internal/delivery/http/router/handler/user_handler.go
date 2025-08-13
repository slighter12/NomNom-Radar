// Package handlers contains the HTTP handlers for the application.
package handler

import (
	"log/slog"
	"net/http"

	"radar/internal/delivery/http/response"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// UserHandler holds dependencies for user-related handlers.
type UserHandler struct {
	uc                 usecase.UserUsecase
	logger             *slog.Logger
	googleOAuthService service.OAuthService
}

// NewUserHandler is the constructor for UserHandler, injected by Fx.
func NewUserHandler(uc usecase.UserUsecase, logger *slog.Logger, googleOAuthService service.OAuthService) *UserHandler {
	return &UserHandler{
		uc:                 uc,
		logger:             logger,
		googleOAuthService: googleOAuthService,
	}
}

// RegisterUser handles the user registration request.
func (h *UserHandler) RegisterUser(c echo.Context) error {
	var input *usecase.RegisterUserInput
	if err := c.Bind(&input); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid registration input")
	}

	output, err := h.uc.RegisterUser(c.Request().Context(), input)
	if err != nil {
		return errors.WithStack(err)
	}

	// Do not return sensitive data in the response.
	// The DTO from the usecase might need to be mapped to a response model.
	return response.Success(c, http.StatusCreated, output.User, "User registered successfully")
}

// RegisterMerchant handles the merchant registration request.
func (h *UserHandler) RegisterMerchant(c echo.Context) error {
	var input *usecase.RegisterMerchantInput
	if err := c.Bind(&input); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid registration input")
	}

	output, err := h.uc.RegisterMerchant(c.Request().Context(), input)
	if err != nil {
		return errors.WithStack(err)
	}

	return response.Success(c, http.StatusCreated, output.User, "Merchant registered successfully")
}

// Login handles the user login request.
func (h *UserHandler) Login(c echo.Context) error {
	var input *usecase.LoginInput
	if err := c.Bind(&input); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid login input")
	}

	output, err := h.uc.Login(c.Request().Context(), input)
	if err != nil {
		return errors.WithStack(err)
	}

	return response.Success(c, http.StatusOK, output, "Login successful")
}

// RefreshToken handles the token refresh request.
func (h *UserHandler) RefreshToken(c echo.Context) error {
	var input *usecase.RefreshTokenInput
	if err := c.Bind(&input); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid refresh token input")
	}

	output, err := h.uc.RefreshToken(c.Request().Context(), input)
	if err != nil {
		return errors.WithStack(err)
	}

	return response.Success(c, http.StatusOK, output, "Token refreshed successfully")
}

// Logout handles the user logout request.
func (h *UserHandler) Logout(c echo.Context) error {
	var input *usecase.LogoutInput
	if err := c.Bind(&input); err != nil {
		return response.BindingError(c, "INVALID_INPUT", "Invalid logout input")
	}

	if err := h.uc.Logout(c.Request().Context(), input); err != nil {
		return errors.WithStack(err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Successfully logged out"}, "Logout successful")
}

// GoogleLogin handles initiating the Google Sign-In flow.
func (h *UserHandler) GoogleLogin(c echo.Context) error {
	// Check if the request wants a redirect or JSON response
	redirect := c.QueryParam("redirect")

	// Use the injected OAuth service to build the authorization URL
	oauthURL := h.googleOAuthService.BuildAuthorizationURL()

	if redirect == "true" {
		// Redirect directly to Google OAuth page
		return c.Redirect(http.StatusTemporaryRedirect, oauthURL)
	}

	// Return JSON response with OAuth URL for frontend use
	return response.Success(c, http.StatusOK, map[string]string{
		"oauth_url":    oauthURL,
		"redirect_url": "/oauth/google?redirect=true",
		"note":         "Use redirect_url for direct redirect, or oauth_url for frontend implementation",
	}, "Google OAuth URL generated successfully")
}

// GoogleCallback handles the Google Sign-In callback.
func (h *UserHandler) GoogleCallback(c echo.Context) error {
	// Check if this is an authorization code callback or ID token callback
	code := c.QueryParam("code")
	idToken := c.FormValue("id_token")

	var input *usecase.GoogleCallbackInput

	if code != "" {
		// Authorization code flow - exchange code for tokens
		// In a real implementation, you would exchange the code for tokens
		// For now, we'll return an error asking for ID token
		return response.BadRequest(c, "INVALID_INPUT", "Authorization code flow not implemented yet. Please use ID token flow.")
	} else if idToken != "" {
		// ID Token flow - verify the token directly
		input = &usecase.GoogleCallbackInput{
			IDToken: idToken,
		}
	} else {
		// Try to bind from JSON body
		if err := c.Bind(&input); err != nil {
			return response.BindingError(c, "INVALID_INPUT", "Invalid Google callback input")
		}
	}

	if input == nil || input.IDToken == "" {
		return response.BadRequest(c, "INVALID_INPUT", "ID token is required")
	}

	output, err := h.uc.GoogleCallback(c.Request().Context(), input)
	if err != nil {
		return errors.WithStack(err)
	}

	return response.Success(c, http.StatusOK, output, "Google OAuth authentication successful")
}

// GetProfile handles the request to get the current user's profile.
func (h *UserHandler) GetProfile(c echo.Context) error {
	userIDVal := c.Get("userID")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	// In a real app, you would have a GetProfile use case
	// user, err := h.uc.GetProfile(c.Request().Context(), userID)

	// For now, just return the ID
	return response.Success(c, http.StatusOK, map[string]string{"message": "Welcome!", "user_id": userID.String()}, "Profile retrieved successfully")
}

// HealthCheck is a simple handler to check if the service is up.
func HealthCheck(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{"status": "ok"}, "Service is healthy")
}
