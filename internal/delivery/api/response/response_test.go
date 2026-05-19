package response

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/platform/observability"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestError_RedactsDetailsForSensitiveStatusCodes(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized},
		{name: "forbidden", statusCode: http.StatusForbidden},
		{name: "internal_server_error", statusCode: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newResponseTestContext()

			err := Error(c, tc.statusCode, "TEST_ERROR", "test message", "sensitive details")
			require.NoError(t, err)

			resp := decodeErrorResponse(t, rec)
			assert.Equal(t, tc.statusCode, rec.Code)
			assert.Equal(t, "TEST_ERROR", resp.Error.Code)
			assert.Equal(t, "test message", resp.Error.Message)
			assert.Nil(t, resp.Error.Details)
			assert.Equal(t, "req-123", resp.Meta.RequestID)
		})
	}
}

func TestError_PreservesDetailsForNonSensitiveClientErrors(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{name: "bad_request", statusCode: http.StatusBadRequest},
		{name: "not_found", statusCode: http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newResponseTestContext()

			err := Error(c, tc.statusCode, "TEST_ERROR", "test message", "client details")
			require.NoError(t, err)

			resp := decodeErrorResponse(t, rec)
			assert.Equal(t, tc.statusCode, rec.Code)
			assert.Equal(t, "client details", resp.Error.Details)
		})
	}
}

func TestAppError_UsesResponseFilteringAndNormalizesEmptyDetails(t *testing.T) {
	testCases := []struct {
		name      string
		appErr    domainerrors.AppError
		wantCode  int
		wantCode2 string
		wantNil   bool
		wantValue string
	}{
		{
			name:      "validation_failed_keeps_details",
			appErr:    domainerrors.ErrValidationFailed.WithDetails("name is required"),
			wantCode:  http.StatusBadRequest,
			wantCode2: "VALIDATION_FAILED",
			wantNil:   false,
			wantValue: "name is required",
		},
		{
			name:      "auth_not_found_keeps_404_details",
			appErr:    domainerrors.ErrAuthNotFound.WithDetails("google auth link missing"),
			wantCode:  http.StatusNotFound,
			wantCode2: "AUTH_NOT_FOUND",
			wantNil:   false,
			wantValue: "google auth link missing",
		},
		{
			name:      "unauthorized_redacts_details",
			appErr:    domainerrors.ErrUnauthorized.WithDetails("token parse failed"),
			wantCode:  http.StatusUnauthorized,
			wantCode2: "UNAUTHORIZED",
			wantNil:   true,
		},
		{
			name:      "empty_details_are_omitted",
			appErr:    domainerrors.ErrNotFound,
			wantCode:  http.StatusNotFound,
			wantCode2: "NOT_FOUND",
			wantNil:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newResponseTestContext()

			err := AppError(c, tc.appErr)
			require.NoError(t, err)

			resp := decodeErrorResponse(t, rec)
			assert.Equal(t, tc.wantCode, rec.Code)
			assert.Equal(t, tc.wantCode2, resp.Error.Code)
			if tc.wantNil {
				assert.Nil(t, resp.Error.Details)
			} else {
				assert.Equal(t, tc.wantValue, resp.Error.Details)
			}
		})
	}
}

func newResponseTestContext() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	ctx := observability.WithCorrelationID(context.Background(), "req-123")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	return c, rec
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	require.NotNil(t, resp.Meta)

	return resp
}
