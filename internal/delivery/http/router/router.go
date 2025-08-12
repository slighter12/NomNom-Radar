// Package router contains routing and server setup for the HTTP delivery.
package router

import (
	"radar/internal/delivery/http/middleware"
	"radar/internal/delivery/http/router/handler"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type RouterParams struct {
	fx.In

	UserHandler    *handler.UserHandler
	AuthMiddleware *middleware.AuthMiddleware
}

// router holds all the handlers that need to be registered.
type router struct {
	userHandler    *handler.UserHandler
	authMiddleware *middleware.AuthMiddleware
}

// NewRouter is the constructor for the Router.
// Fx will inject the required handlers here.
func NewRouter(params RouterParams) *router {
	return &router{
		userHandler:    params.UserHandler,
		authMiddleware: params.AuthMiddleware,
	}
}

// RegisterRoutes sets up all the API routes for the application.
func (r *router) RegisterRoutes(e *echo.Echo) {
	// Health check endpoint
	e.GET("/health", handler.HealthCheck)

	// Auth routes
	authGroup := e.Group("/auth")
	{
		authGroup.POST("/register/user", r.userHandler.RegisterUser)
		// authGroup.POST("/register/merchant", r.userHandler.RegisterMerchant)
		authGroup.POST("/login", r.userHandler.Login)
		// ... add refresh, logout routes etc.
	}

	// User routes that require authentication
	userGroup := e.Group("/user")
	userGroup.Use(r.authMiddleware.Authenticate) // Apply JWT authentication middleware
	{
		userGroup.GET("/profile", r.userHandler.GetProfile)
	}

	// Merchant routes that require authentication and "merchant" role
	merchantGroup := e.Group("/merchant")
	merchantGroup.Use(r.authMiddleware.Authenticate)            // First, check if logged in
	merchantGroup.Use(r.authMiddleware.RequireRole("merchant")) // Then, check for the role
	{
		// ... merchant-specific handlers
		// merchantGroup.GET("/dashboard", r.merchantHandler.GetDashboard)
	}
}
