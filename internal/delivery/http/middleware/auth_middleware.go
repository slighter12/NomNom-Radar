package middleware

import (
	"net/http"
	"slices"
	"strings"

	"radar/config"
	"radar/internal/domain/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization header is missing"})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token format, must be Bearer token"})
		}

		token, err := m.tokenSvc.ValidateToken(tokenString, m.cfg.SecretKey.Access)
		if err != nil || !token.Valid {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Failed to parse token claims"})
		}

		// Extract user ID
		userIDStr, ok := claims["sub"].(string)
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "User ID missing from token"})
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid user ID format in token"})
		}

		// Extract roles
		rolesClaim, _ := claims["roles"].([]any)
		var roles []string
		for _, r := range rolesClaim {
			if roleStr, ok := r.(string); ok {
				roles = append(roles, roleStr)
			}
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
				return c.JSON(http.StatusForbidden, map[string]string{"error": "Permission denied: role information missing"})
			}

			if !slices.Contains(roles, requiredRole) {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "Permission denied: require '" + requiredRole + "' role"})
			}

			return next(c)
		}
	}
}
