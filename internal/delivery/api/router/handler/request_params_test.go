package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/internal/delivery"
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

	require.ErrorIs(t, err, delivery.ErrResponseHandled)
	assert.Nil(t, payload)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_ERROR"`)
	assert.Contains(t, rec.Body.String(), `"message":"name is required"`)
}

func TestBindAndValidateRequest_StopsAfterValidationError(t *testing.T) {
	c, rec := newJSONContext(http.MethodPost, "/", `{}`)
	var payload testValidationPayload

	err := bindAndValidateRequest(c, &payload, "Invalid input")

	require.ErrorIs(t, err, delivery.ErrResponseHandled)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_ERROR"`)
	assert.Contains(t, rec.Body.String(), `"message":"name is required"`)
}
