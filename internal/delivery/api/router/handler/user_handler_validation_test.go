package handler

import (
	"net/http"
	"testing"

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
		wantCode   string
		wantDetail string
	}{
		{
			name:       "register user missing name",
			target:     "/register",
			body:       `{"email":"user@example.com","password":"secret"}`,
			handle:     handler.RegisterUser,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "name is required",
		},
		{
			name:       "login missing email",
			target:     "/login",
			body:       `{"password":"secret"}`,
			handle:     handler.Login,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "email is required",
		},
		{
			name:       "refresh token missing refresh token",
			target:     "/refresh",
			body:       `{}`,
			handle:     handler.RefreshToken,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "refresh_token is required",
		},
		{
			name:       "logout missing refresh token",
			target:     "/logout",
			body:       `{}`,
			handle:     handler.Logout,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "refresh_token is required",
		},
		{
			name:       "google callback missing id token",
			target:     "/google/callback",
			body:       `{}`,
			handle:     handler.GoogleCallback,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "id_token is required",
		},
		{
			name:       "google callback invalid query state",
			target:     "/google/callback?state=admin",
			body:       `{"id_token":"token","requested_role":"user"}`,
			handle:     handler.GoogleCallback,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "state must be one of [user merchant]",
		},
		{
			name:       "merchant onboarding missing store name",
			target:     "/merchant/onboarding",
			body:       `{"onboarding_token":"token"}`,
			handle:     handler.CompleteMerchantOnboarding,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "store_name is required",
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
			wantCode:   "VALIDATION_FAILED",
			wantDetail: "business_license is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newJSONContext(http.MethodPost, tt.target, tt.body)

			err := tt.handle(c)
			writeTestErrorResponse(c, err)

			require.Error(t, err)
			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"`+tt.wantCode+`"`)
			assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
			assert.Contains(t, rec.Body.String(), `"details":"`+tt.wantDetail+`"`)
		})
	}
}

func TestUserHandler_UpdateMerchantDiscoveryProfile_InvalidUUIDRejectedAtHTTPBoundary(t *testing.T) {
	handler := &UserHandler{}
	c, rec := newJSONContext(http.MethodPatch, "/merchant/discovery-profile", `{"discovery_category_id":"not-a-uuid"}`)
	c.Set("userID", uuid.New())

	err := handler.UpdateMerchantDiscoveryProfile(c)
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"INVALID_INPUT"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入格式錯誤"`)
	assert.Contains(t, rec.Body.String(), `"details":"Invalid merchant discovery profile input"`)
}
