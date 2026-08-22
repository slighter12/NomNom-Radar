package middleware

import (
	"encoding/json"
	"strings"

	"radar/config"
	"radar/internal/delivery/api/response"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"
	"radar/internal/usecase"

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

// RefreshTokenSubject returns the user in the request's refresh token, if any.
func (m *AuthMiddleware) RefreshTokenSubject(c echo.Context) (uuid.UUID, bool) {
	if c == nil {
		return uuid.Nil, false
	}

	rawBody, ok := c.Get(capturedRequestBodyKey).([]byte)
	if !ok || len(rawBody) == 0 {
		return uuid.Nil, false
	}

	var input usecase.RefreshTokenInput
	if err := json.Unmarshal(rawBody, &input); err != nil || input.RefreshToken == "" {
		return uuid.Nil, false
	}

	claims, err := m.tokenSvc.ValidateToken(input.RefreshToken)
	if err != nil || claims == nil || claims.Type != service.TokenTypeRefresh {
		return uuid.Nil, false
	}

	return claims.UserID, true
}

// Authenticate is the core middleware function that validates the JWT access token.
func (m *AuthMiddleware) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return response.AuthRequired(c)
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return response.InvalidToken(c)
		}

		claims, err := m.tokenSvc.ValidateToken(tokenString)
		if err != nil {
			return response.InvalidToken(c)
		}
		if claims.Type != service.TokenTypeAccess {
			return response.InvalidToken(c)
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
				return response.ForbiddenAccess(c)
			}

			if !roles.Contains(requiredRole) {
				return response.ForbiddenAccess(c)
			}

			return next(c)
		}
	}
}
