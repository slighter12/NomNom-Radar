package handler

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	apimiddleware "radar/internal/delivery/api/middleware"
	apivalidator "radar/internal/delivery/api/validator"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testValidationPayload struct {
	Name string `json:"name" validate:"required"`
}

func newJSONContext(method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = apivalidator.New()

	req := httptest.NewRequestWithContext(context.Background(), method, target, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	return e.NewContext(req, rec), rec
}

func TestBindRequiredPayload_ValidatesPayload(t *testing.T) {
	c, rec := newJSONContext(http.MethodPost, "/", `{}`)

	payload, err := bindRequiredPayload[testValidationPayload](c, "Invalid input")
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Nil(t, payload)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
	assert.Contains(t, rec.Body.String(), `"details":"name is required"`)
}

func TestBindAndValidateRequest_StopsAfterValidationError(t *testing.T) {
	c, rec := newJSONContext(http.MethodPost, "/", `{}`)
	var payload testValidationPayload

	err := bindAndValidateRequest(c, &payload, "Invalid input")
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	assert.Contains(t, rec.Body.String(), `"message":"輸入資料驗證失敗"`)
	assert.Contains(t, rec.Body.String(), `"details":"name is required"`)
}

func writeTestErrorResponse(c echo.Context, err error) {
	if err == nil {
		return
	}

	apimiddleware.NewErrorMiddleware(slog.Default()).HandleHTTPError(err, c)
}
