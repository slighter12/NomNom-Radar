// Package router contains routing and server setup for the HTTP delivery.
package router

import (
	"radar/config"
	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router/handler"
	"radar/internal/domain/entity"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type RouterParams struct {
	fx.In

	UserHandler         *handler.UserHandler
	TestHandler         *handler.TestHandler
	LocationHandler     *handler.LocationHandler
	DeviceHandler       *handler.DeviceHandler
	SubscriptionHandler *handler.SubscriptionHandler
	NotificationHandler *handler.NotificationHandler
	AuthMiddleware      *middleware.AuthMiddleware
	Config              *config.Config
}

// router holds all the handlers that need to be registered.
type router struct {
	userHandler         *handler.UserHandler
	testHandler         *handler.TestHandler
	locationHandler     *handler.LocationHandler
	deviceHandler       *handler.DeviceHandler
	subscriptionHandler *handler.SubscriptionHandler
	notificationHandler *handler.NotificationHandler
	authMiddleware      *middleware.AuthMiddleware
	config              *config.Config
}

// NewRouter is the constructor for the Router.
// Fx will inject the required handlers here.
func NewRouter(params RouterParams) *router {
	return &router{
		userHandler:         params.UserHandler,
		testHandler:         params.TestHandler,
		locationHandler:     params.LocationHandler,
		deviceHandler:       params.DeviceHandler,
		subscriptionHandler: params.SubscriptionHandler,
		notificationHandler: params.NotificationHandler,
		authMiddleware:      params.AuthMiddleware,
		config:              params.Config,
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
		authGroup.POST("/register/merchant", r.userHandler.RegisterMerchant)
		authGroup.POST("/login", r.userHandler.Login)
		authGroup.POST("/refresh", r.userHandler.RefreshToken)
		authGroup.POST("/logout", r.userHandler.Logout)
	}

	// OAuth routes - separate group for better organization
	oauthGroup := e.Group("/oauth")
	{
		oauthGroup.POST("/google/callback", r.userHandler.GoogleCallback) // Handle callback
	}

	// User routes that require authentication
	userGroup := e.Group("/user")
	userGroup.Use(r.authMiddleware.Authenticate) // Apply JWT authentication middleware
	{
		userGroup.GET("/profile", r.userHandler.GetProfile)
	}

	// Merchant routes that require authentication and "merchant" role
	merchantGroup := e.Group("/merchant")
	merchantGroup.Use(r.authMiddleware.Authenticate)                     // First, check if logged in
	merchantGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant)) // Then, check for the role
	{
		// ... merchant-specific handlers
		// merchantGroup.GET("/dashboard", r.merchantHandler.GetDashboard)
	}

	// API v1 routes
	apiV1 := e.Group("/api/v1")
	apiV1.Use(r.authMiddleware.Authenticate) // All API v1 routes require authentication

	// Location management routes
	locationsGroup := apiV1.Group("/locations")
	{
		// User location routes
		locationsGroup.POST("/user", r.locationHandler.CreateUserLocation)
		locationsGroup.GET("/user", r.locationHandler.GetUserLocations)
		locationsGroup.PUT("/user/:id", r.locationHandler.UpdateUserLocation)
		locationsGroup.DELETE("/user/:id", r.locationHandler.DeleteUserLocation)

		// Merchant location routes (require merchant role)
		merchantLocGroup := locationsGroup.Group("/merchant")
		merchantLocGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant))
		{
			merchantLocGroup.POST("", r.locationHandler.CreateMerchantLocation)
			merchantLocGroup.GET("", r.locationHandler.GetMerchantLocations)
			merchantLocGroup.PUT("/:id", r.locationHandler.UpdateMerchantLocation)
			merchantLocGroup.DELETE("/:id", r.locationHandler.DeleteMerchantLocation)
		}
	}

	// Device management routes
	devicesGroup := apiV1.Group("/devices")
	{
		devicesGroup.POST("", r.deviceHandler.RegisterDevice)
		devicesGroup.GET("", r.deviceHandler.GetUserDevices)
		devicesGroup.PUT("/:id/token", r.deviceHandler.UpdateFCMToken)
		devicesGroup.DELETE("/:id", r.deviceHandler.DeactivateDevice)
	}

	// Subscription management routes
	subscriptionsGroup := apiV1.Group("/subscriptions")
	{
		subscriptionsGroup.POST("", r.subscriptionHandler.SubscribeToMerchant)
		subscriptionsGroup.DELETE("/:merchantId", r.subscriptionHandler.UnsubscribeFromMerchant)
		subscriptionsGroup.GET("", r.subscriptionHandler.GetUserSubscriptions)
		subscriptionsGroup.POST("/qr", r.subscriptionHandler.ProcessQRSubscription)
	}

	// Merchant QR code generation (requires merchant role)
	merchantsGroup := apiV1.Group("/merchants")
	merchantsGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant))
	{
		merchantsGroup.GET("/:id/qr", r.subscriptionHandler.GenerateSubscriptionQR)
	}

	// Notification management routes (require merchant role)
	notificationsGroup := apiV1.Group("/notifications")
	notificationsGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant))
	{
		notificationsGroup.POST("", r.notificationHandler.PublishLocationNotification)
		notificationsGroup.GET("", r.notificationHandler.GetMerchantNotificationHistory)
	}
}

func (r *router) RegisterTestRoutes(e *echo.Echo) {
	// Test routes - only enabled when configured
	if r.config.TestRoutes != nil && r.config.TestRoutes.Enabled {
		// Test routes that require authentication
		testGroup := e.Group("/test")
		testGroup.GET("/public", r.testHandler.TestPublicEndpoint)

		testGroup.Use(r.authMiddleware.Authenticate) // Apply JWT authentication middleware
		{
			testGroup.GET("/auth", r.testHandler.TestAuthMiddleware)
		}
	}
}
