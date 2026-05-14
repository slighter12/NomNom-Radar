package handler

import (
	"net/http"
	"testing"

	"radar/internal/delivery"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_InvalidPayloadsAreRejectedAtHTTPBoundary(t *testing.T) {
	handler := &UserHandler{}

	tests := []struct {
		name       string
		target     string
		body       string
		handle     func(echo.Context) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "register user missing name",
			target:     "/register",
			body:       `{"email":"user@example.com","password":"secret"}`,
			handle:     handler.RegisterUser,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"name is required"`,
		},
		{
			name:       "login missing email",
			target:     "/login",
			body:       `{"password":"secret"}`,
			handle:     handler.Login,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"email is required"`,
		},
		{
			name:       "refresh token missing refresh token",
			target:     "/refresh",
			body:       `{}`,
			handle:     handler.RefreshToken,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"refresh_token is required"`,
		},
		{
			name:       "logout missing refresh token",
			target:     "/logout",
			body:       `{}`,
			handle:     handler.Logout,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"refresh_token is required"`,
		},
		{
			name:       "google callback missing id token",
			target:     "/google/callback",
			body:       `{}`,
			handle:     handler.GoogleCallback,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"id_token is required"`,
		},
		{
			name:       "google callback invalid query state",
			target:     "/google/callback?state=admin",
			body:       `{"id_token":"token","requested_role":"user"}`,
			handle:     handler.GoogleCallback,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"state must be one of [user merchant]"`,
		},
		{
			name:       "merchant onboarding missing store name",
			target:     "/merchant/onboarding",
			body:       `{"onboarding_token":"token"}`,
			handle:     handler.CompleteMerchantOnboarding,
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"store_name is required"`,
		},
		{
			name:   "merchant verification missing business license",
			target: "/merchant/verification",
			body:   `{}`,
			handle: func(c echo.Context) error {
				c.Set("userID", uuid.New())

				return handler.SubmitMerchantVerification(c)
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   `"message":"business_license is required"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newJSONContext(http.MethodPost, tt.target, tt.body)

			err := tt.handle(c)

			require.ErrorIs(t, err, delivery.ErrResponseHandled)
			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_ERROR"`)
			assert.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}
}

func TestUserHandler_UpdateMerchantDiscoveryProfile_InvalidUUIDRejectedAtHTTPBoundary(t *testing.T) {
	handler := &UserHandler{}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/discovery-profile", `{"discovery_category_id":"not-a-uuid"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantDiscoveryProfile(c)

	require.ErrorIs(t, err, delivery.ErrResponseHandled)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"INVALID_INPUT"`)
	assert.Contains(t, rec.Body.String(), `"message":"Invalid merchant discovery profile input"`)
}
