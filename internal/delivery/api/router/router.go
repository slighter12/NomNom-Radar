package router

import (
	"fmt"

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
	MenuHandler         *handler.MenuHandler
	DeviceHandler       *handler.DeviceHandler
	SubscriptionHandler *handler.SubscriptionHandler
	NotificationHandler *handler.NotificationHandler
	DiscoveryHandler    *handler.DiscoveryHandler
	AuthMiddleware      *middleware.AuthMiddleware
	Config              *config.Config
}

// router holds all the handlers that need to be registered.
type router struct {
	userHandler         *handler.UserHandler
	testHandler         *handler.TestHandler
	locationHandler     *handler.LocationHandler
	menuHandler         *handler.MenuHandler
	deviceHandler       *handler.DeviceHandler
	subscriptionHandler *handler.SubscriptionHandler
	notificationHandler *handler.NotificationHandler
	discoveryHandler    *handler.DiscoveryHandler
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
		menuHandler:         params.MenuHandler,
		deviceHandler:       params.DeviceHandler,
		subscriptionHandler: params.SubscriptionHandler,
		notificationHandler: params.NotificationHandler,
		discoveryHandler:    params.DiscoveryHandler,
		authMiddleware:      params.AuthMiddleware,
		config:              params.Config,
	}
}

// RegisterRoutes sets up all the API routes for the application.
func (r *router) RegisterRoutes(e *echo.Echo) error {
	// Health check endpoint
	e.GET("/health", handler.HealthCheck)

	if err := r.registerPublicRoutes(e); err != nil {
		return err
	}
	if err := r.registerAPIV1Routes(e); err != nil {
		return err
	}

	return nil
}

func (r *router) registerPublicRoutes(e *echo.Echo) error {
	authRateLimiter, err := middleware.NewAuthRateLimiter(r.config)
	if err != nil {
		return fmt.Errorf("configure auth rate limiter: %w", err)
	}
	sessionRateLimiter, err := middleware.NewSessionRateLimiter(r.config, r.authMiddleware.RefreshTokenSubject)
	if err != nil {
		return fmt.Errorf("configure session rate limiter: %w", err)
	}

	credentialGroup := e.Group("/auth")
	if authRateLimiter != nil {
		credentialGroup.Use(authRateLimiter)
	}
	{
		credentialGroup.POST("/register/user", r.userHandler.RegisterUser)
		credentialGroup.POST("/register/merchant", r.userHandler.RegisterMerchant)
		credentialGroup.POST("/login", r.userHandler.Login)
		credentialGroup.POST("/onboarding/merchant", r.userHandler.CompleteMerchantOnboarding)
		credentialGroup.POST("/link-provider", r.userHandler.LinkProvider)
	}

	sessionGroup := e.Group("/auth")
	if sessionRateLimiter != nil {
		sessionGroup.Use(sessionRateLimiter)
	}
	{
		sessionGroup.POST("/refresh", r.userHandler.RefreshToken)
		sessionGroup.POST("/logout", r.userHandler.Logout)
	}

	oauthGroup := e.Group("/oauth")
	if authRateLimiter != nil {
		oauthGroup.Use(authRateLimiter)
	}
	{
		oauthGroup.POST("/google/callback", r.userHandler.GoogleCallback)
	}

	return nil
}

func (r *router) registerAPIV1Routes(e *echo.Echo) error {
	apiRateLimiter, err := middleware.NewAPIRateLimiter(r.config)
	if err != nil {
		return fmt.Errorf("configure API rate limiter: %w", err)
	}

	apiV1 := e.Group("/api/v1")
	apiV1.Use(r.authMiddleware.Authenticate)
	if apiRateLimiter != nil {
		apiV1.Use(apiRateLimiter)
	}

	r.registerAPIV1UserRoutes(apiV1)
	r.registerAPIV1SharedRoutes(apiV1)
	r.registerAPIV1ConsumerRoutes(apiV1)
	r.registerAPIV1MerchantRoutes(apiV1)

	return nil
}

func (r *router) registerAPIV1UserRoutes(apiV1 *echo.Group) {
	userGroup := apiV1.Group("/user")
	{
		userGroup.GET("/profile", r.userHandler.GetProfile)
	}

	locationsGroup := apiV1.Group("/locations")
	{
		locationsGroup.POST("/user", r.locationHandler.CreateUserLocation)
		locationsGroup.GET("/user", r.locationHandler.GetUserLocations)
		locationsGroup.PUT("/user/:locationId", r.locationHandler.UpdateUserLocation)
		locationsGroup.DELETE("/user/:locationId", r.locationHandler.DeleteUserLocation)
	}

	devicesGroup := apiV1.Group("/devices")
	{
		devicesGroup.POST("", r.deviceHandler.RegisterDevice)
		devicesGroup.GET("", r.deviceHandler.GetUserDevices)
		devicesGroup.GET("/health", r.deviceHandler.GetDeviceHealth)
		devicesGroup.PUT("/:deviceId/token", r.deviceHandler.UpdateFCMToken)
		devicesGroup.DELETE("/:deviceId", r.deviceHandler.DeactivateDevice)
	}

	subscriptionsGroup := apiV1.Group("/subscriptions")
	{
		subscriptionsGroup.POST("", r.subscriptionHandler.SubscribeToMerchant)
		subscriptionsGroup.DELETE("/:merchantId", r.subscriptionHandler.UnsubscribeFromMerchant)
		subscriptionsGroup.GET("", r.subscriptionHandler.GetUserSubscriptions)
		subscriptionsGroup.POST("/qr", r.subscriptionHandler.ProcessQRSubscription)
	}
}

func (r *router) registerAPIV1SharedRoutes(apiV1 *echo.Group) {
	discoveryGroup := apiV1.Group("/discovery")
	{
		discoveryGroup.GET("/categories", r.discoveryHandler.ListActiveCategories)
		discoveryGroup.GET("/subcategories", r.discoveryHandler.ListActiveSubcategories)
		discoveryGroup.GET("/hubs", r.discoveryHandler.ListActiveHubs)
	}

	locationsGroup := apiV1.Group("/locations/merchant")
	locationsGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant))
	{
		locationsGroup.POST("", r.locationHandler.CreateMerchantLocation)
		locationsGroup.GET("", r.locationHandler.GetMerchantLocations)
		locationsGroup.PUT("/:locationId", r.locationHandler.UpdateMerchantLocation)
		locationsGroup.DELETE("/:locationId", r.locationHandler.DeleteMerchantLocation)
	}
}

func (r *router) registerAPIV1ConsumerRoutes(apiV1 *echo.Group) {
	// These endpoints are intentionally limited to authenticated user-role accounts.
	// Merchant-only accounts are excluded even though the menu data is consumer-visible.
	consumerMerchantsGroup := apiV1.Group("/merchants")
	consumerMerchantsGroup.Use(r.authMiddleware.RequireRole(entity.RoleUser))
	{
		consumerMerchantsGroup.GET("", r.discoveryHandler.SearchPublicMerchants)
		consumerMerchantsGroup.GET("/:merchantId/menu", r.menuHandler.GetPublicMerchantMenu)
	}
}

func (r *router) registerAPIV1MerchantRoutes(apiV1 *echo.Group) {
	merchantGroup := apiV1.Group("/merchant")
	merchantGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant))
	{
		merchantGroup.GET("/qr", r.subscriptionHandler.GenerateSubscriptionQR)
		merchantGroup.POST("/verification", r.userHandler.SubmitMerchantVerification)
		merchantGroup.PATCH("/profile", r.userHandler.UpdateMerchantProfile)
		merchantGroup.GET("/discovery-profile", r.userHandler.GetMerchantDiscoveryProfile)
		merchantGroup.PATCH("/discovery-profile", r.userHandler.UpdateMerchantDiscoveryProfile)
	}

	merchantMenusGroup := apiV1.Group("/menus/merchant")
	merchantMenusGroup.Use(r.authMiddleware.RequireRole(entity.RoleMerchant))
	{
		merchantMenusGroup.GET("", r.menuHandler.GetMerchantMenuItems)
		merchantMenusGroup.POST("", r.menuHandler.CreateMenuItem)
		merchantMenusGroup.PATCH("/reorder", r.menuHandler.ReorderMenuItems)
		merchantMenusGroup.PUT("/:menuItemId", r.menuHandler.UpdateMenuItem)
		merchantMenusGroup.PATCH("/:menuItemId/status", r.menuHandler.UpdateMenuItemStatus)
		merchantMenusGroup.DELETE("/:menuItemId", r.menuHandler.DeleteMenuItem)
	}

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
