package middleware

import (
	"slices"
	"strings"

	"radar/config"
	"radar/internal/delivery/http/response"
	"radar/internal/domain/service"

	"github.com/labstack/echo/v4"
)

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

		// Extract roles
		var roles []string
		if claims.Roles != nil {
			roles = claims.Roles
		}

		// Set user info on the context for handlers to use
		c.Set("userID", userID)
		c.Set("roles", roles)

		return next(c)
	}
}

// RequireRole is a middleware factory that checks if the user has a specific role.
// It must be used AFTER the Authenticate middleware.
func (m *AuthMiddleware) RequireRole(requiredRole string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rolesVal := c.Get("roles")
			roles, ok := rolesVal.([]string)
			if !ok {
				return response.Forbidden(c, "FORBIDDEN", "Permission denied: role information missing")
			}

			if !slices.Contains(roles, requiredRole) {
				return response.Forbidden(c, "FORBIDDEN", "Permission denied: require '"+requiredRole+"' role")
			}

			return next(c)
		}
	}
}
