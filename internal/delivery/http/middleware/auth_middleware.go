package middleware

import (
	"strings"

	"radar/config"
	"radar/internal/delivery/http/response"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const (
	// contextKeyUserID is the key for storing user ID in context.
	contextKeyUserID ContextKey = "userID"
	// contextKeyRoles is the key for storing user roles in context.
	contextKeyRoles ContextKey = "roles"
)

// GetUserID extracts the authenticated user ID from context.
// Returns the user ID and a boolean indicating success.
func GetUserID(c echo.Context) (uuid.UUID, bool) {
	val := c.Get(string(contextKeyUserID))
	id, ok := val.(uuid.UUID)

	return id, ok
}

// GetRoles extracts the user roles from context.
// Returns the roles and a boolean indicating success.
func GetRoles(c echo.Context) (entity.Roles, bool) {
	val := c.Get(string(contextKeyRoles))
	roles, ok := val.(entity.Roles)

	return roles, ok
}

// AuthMiddleware provides middleware for JWT authentication and authorization.
type AuthMiddleware struct {
	tokenSvc service.TokenService
	cfg      *config.Config
}

// NewAuthMiddleware is the constructor for AuthMiddleware.
func NewAuthMiddleware(tokenSvc service.TokenService, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{tokenSvc: tokenSvc, cfg: cfg}
}

// Authenticate is the core middleware function that validates the JWT access token.
func (m *AuthMiddleware) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "UNAUTHORIZED", "Authorization header is missing")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return response.Unauthorized(c, "INVALID_TOKEN", "Invalid token format, must be Bearer token")
		}

		claims, err := m.tokenSvc.ValidateToken(tokenString)
		if err != nil {
			return response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired token")
		}

		// Extract user ID
		userID := claims.UserID

		// Convert []string roles from JWT to entity.Roles (boundary conversion)
		roles := entity.RolesFromStrings(claims.Roles)

		// Set user info on the context for handlers to use
		c.Set(string(contextKeyUserID), userID)
		c.Set(string(contextKeyRoles), roles)

		return next(c)
	}
}

// RequireRole is a middleware factory that checks if the user has a specific role.
// It must be used AFTER the Authenticate middleware.
func (m *AuthMiddleware) RequireRole(requiredRole entity.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			roles, ok := GetRoles(c)
			if !ok {
				return response.Forbidden(c, "FORBIDDEN", "Permission denied: role information missing")
			}

			if !roles.Contains(requiredRole) {
				return response.Forbidden(c, "FORBIDDEN", "Permission denied: require '"+requiredRole.String()+"' role")
			}

			return next(c)
		}
	}
}
