package handler

import (
	"net/http"

	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/response"

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
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "CONTEXT_ERROR", "User ID not found in context")
	}

	roles, ok := middleware.GetRoles(c)
	if !ok {
		return response.Unauthorized(c, "CONTEXT_ERROR", "User roles not found in context")
	}

	return response.Success(c, http.StatusOK, map[string]any{
		"message": "Authentication middleware test successful",
		"userID":  userID,
		"roles":   roles,
		"status":  "authenticated",
	})
}

// TestPublicEndpoint tests a public endpoint (no authentication required)
func (h *TestHandler) TestPublicEndpoint(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]any{
		"message": "Public endpoint test successful",
		"status":  "public",
	})
}
