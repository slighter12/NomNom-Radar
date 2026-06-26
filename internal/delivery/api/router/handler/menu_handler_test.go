package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMenuHandler_ParseListMerchantMenuItemsInput_ParsesCategoryID(t *testing.T) {
	categoryID := uuid.New()
	handler := &MenuHandler{}
	c, _ := newJSONContext(http.MethodGet, "/menus/merchant?category_id="+categoryID.String()+"&keyword=%20noodles%20", "")

	input, err := handler.parseListMerchantMenuItemsInput(c)

	require.NoError(t, err)
	require.NotNil(t, input.CategoryID)
	assert.Equal(t, categoryID, *input.CategoryID)
	assert.Equal(t, "noodles", input.Keyword)
}

func TestMenuHandler_ValidateCreateMenuItemRequest_RequiresCategoryID(t *testing.T) {
	handler := &MenuHandler{}
	req := &CreateMenuItemRequest{
		Name:        "Noodles",
		Price:       120,
		Currency:    "TWD",
		PrepMinutes: 10,
	}
	c, rec := newJSONContext(http.MethodPost, "/menus/merchant", "")

	err := handler.validateCreateMenuItemRequest(c, req)
	writeTestErrorResponse(c, err)

	require.Error(t, err)
	assert.Contains(t, rec.Body.String(), "category_id")
}
