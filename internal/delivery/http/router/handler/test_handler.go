package handler

import (
	"net/http"

	"radar/internal/delivery/http/response"

	"github.com/labstack/echo/v4"
)

// TestHandler handles test endpoints for middleware validation
type TestHandler struct{}

// NewTestHandler creates a new TestHandler instance
func NewTestHandler() *TestHandler {
	return &TestHandler{}
}

// TestAuthMiddleware tests the authentication middleware
// This endpoint requires a valid JWT token in the Authorization header
func (h *TestHandler) TestAuthMiddleware(c echo.Context) error {
	// Get user information from context (set by auth middleware)
	userID := c.Get("userID")
	roles := c.Get("roles")

	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Authentication middleware test successful",
		"userID":  userID,
		"roles":   roles,
		"status":  "authenticated",
	}, "Authentication middleware test successful")
}

// TestPublicEndpoint tests a public endpoint (no authentication required)
func (h *TestHandler) TestPublicEndpoint(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Public endpoint test successful",
		"status":  "public",
	}, "Public endpoint test successful")
}
