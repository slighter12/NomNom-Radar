package router

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"radar/config"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router/handler"
	apivalidator "radar/internal/delivery/api/validator"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUserToken     = "user-token"
	testMerchantToken = "merchant-token"
)

type routerTestTokenService struct {
	claims map[string]*service.Claims
}

func (s *routerTestTokenService) GenerateTokens(uuid.UUID, []string) (string, string, error) {
	return "", "", nil
}

func (s *routerTestTokenService) ValidateToken(token string) (*service.Claims, error) {
	claims, ok := s.claims[token]
	if !ok {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func (s *routerTestTokenService) GenerateOnboardingToken(uuid.UUID) (string, error) {
	return "", nil
}

func (s *routerTestTokenService) GenerateLinkingToken(uuid.UUID, string, string, string, string) (string, error) {
	return "", nil
}

func (s *routerTestTokenService) GetRefreshTokenDuration() time.Duration {
	return time.Hour
}

func (s *routerTestTokenService) HashToken(token string) string {
	return token
}

func (s *routerTestTokenService) RotateTokens(uuid.UUID, []string) (string, string, string, error) {
	return "", "", "", nil
}

type routerTestDiscoveryUsecase struct{}

func (uc *routerTestDiscoveryUsecase) ListActiveCategories(context.Context) (*usecase.ListDiscoveryCategoriesResult, error) {
	return &usecase.ListDiscoveryCategoriesResult{
		Categories: []*usecase.DiscoveryCategoryResult{{ID: uuid.New(), Slug: "food", Name: "Food"}},
	}, nil
}

func (uc *routerTestDiscoveryUsecase) ListActiveSubcategories(context.Context) (*usecase.ListDiscoverySubcategoriesResult, error) {
	return &usecase.ListDiscoverySubcategoriesResult{
		Subcategories: []*usecase.DiscoverySubcategoryResult{{ID: uuid.New(), Slug: "meal"}},
	}, nil
}

func (uc *routerTestDiscoveryUsecase) ListActiveHubs(context.Context) (*usecase.ListDiscoveryHubsResult, error) {
	return &usecase.ListDiscoveryHubsResult{
		Hubs: []*usecase.DiscoveryHubResult{{ID: uuid.New(), Slug: "night-market"}},
	}, nil
}

func (uc *routerTestDiscoveryUsecase) SearchPublicMerchants(
	_ context.Context,
	input *usecase.SearchPublicMerchantsInput,
) (*usecase.SearchPublicMerchantsResult, error) {
	return &usecase.SearchPublicMerchantsResult{
		Merchants: []*entity.PublicMerchantSearchItem{},
		Pagination: &usecase.MerchantSearchPagination{
			Page:     input.Page,
			PageSize: input.PageSize,
			Total:    0,
		},
	}, nil
}

type routerTestProfileUsecase struct{}

func (uc *routerTestProfileUsecase) GetProfile(_ context.Context, userID uuid.UUID) (*entity.User, error) {
	return &entity.User{ID: userID, Email: "tester@example.com"}, nil
}

func (uc *routerTestProfileUsecase) UpdateUserProfile(context.Context, uuid.UUID, *usecase.UpdateUserProfileInput) error {
	return nil
}

func (uc *routerTestProfileUsecase) UpdateMerchantProfile(context.Context, uuid.UUID, *usecase.UpdateMerchantProfileInput) error {
	return nil
}

func (uc *routerTestProfileUsecase) GetMerchantDiscoveryProfile(context.Context, uuid.UUID) (*usecase.MerchantDiscoveryProfileResult, error) {
	return &usecase.MerchantDiscoveryProfileResult{}, nil
}

func (uc *routerTestProfileUsecase) UpdateMerchantDiscoveryProfile(
	context.Context,
	uuid.UUID,
	*usecase.UpdateMerchantDiscoveryProfileInput,
) (*usecase.MerchantDiscoveryProfileResult, error) {
	return &usecase.MerchantDiscoveryProfileResult{}, nil
}

func (uc *routerTestProfileUsecase) SubmitMerchantVerification(context.Context, uuid.UUID, *usecase.SubmitMerchantVerificationInput) error {
	return nil
}

func (uc *routerTestProfileUsecase) SwitchToMerchant(context.Context, uuid.UUID, *usecase.SwitchToMerchantInput) error {
	return nil
}

func (uc *routerTestProfileUsecase) GetUserRole(context.Context, uuid.UUID) ([]string, error) {
	return nil, nil
}

func TestRouter_DiscoveryValuesAllowMerchantRole(t *testing.T) {
	e := newRouterTestEcho()

	for _, tt := range []struct {
		path string
		key  string
	}{
		{path: "/api/v1/discovery/categories", key: `"categories"`},
		{path: "/api/v1/discovery/subcategories", key: `"subcategories"`},
		{path: "/api/v1/discovery/hubs", key: `"hubs"`},
	} {
		t.Run(tt.path, func(t *testing.T) {
			req := newRouterTestRequest(http.MethodGet, tt.path, testMerchantToken)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.key)
		})
	}
}

func TestRouter_DiscoveryValuesRequireAuthentication(t *testing.T) {
	e := newRouterTestEcho()

	for _, path := range []string{
		"/api/v1/discovery/categories",
		"/api/v1/discovery/subcategories",
		"/api/v1/discovery/hubs",
	} {
		t.Run(path, func(t *testing.T) {
			req := newRouterTestRequest(http.MethodGet, path, "")
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"UNAUTHORIZED"`)
		})
	}
}

func TestRouter_APIV1AuthenticationRunsBeforeRateLimiter(t *testing.T) {
	rate := 0.001
	burst := 1
	expiresIn := time.Minute
	cfg := &config.Config{}
	cfg.HTTP.APIRateLimit = &config.RateLimitConfig{
		Rate:      &rate,
		Burst:     &burst,
		ExpiresIn: &expiresIn,
	}
	e := newRouterTestEchoWithConfig(cfg)

	for i := range 2 {
		req := newRouterTestRequest(http.MethodGet, "/api/v1/discovery/categories", "")
		req.RemoteAddr = "192.0.2.1:1000"
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, "request %d should be rejected by authentication", i+1)
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	}
}

func TestRouter_PublicMerchantSearchStillRequiresUserRole(t *testing.T) {
	e := newRouterTestEcho()
	req := newRouterTestRequest(http.MethodGet, "/api/v1/merchants", testMerchantToken)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"FORBIDDEN"`)
}

func TestRouter_ProfileSupportsAPIV1AndLegacyRoutes(t *testing.T) {
	e := newRouterTestEcho()

	for _, path := range []string{"/api/v1/user/profile", "/user/profile"} {
		t.Run(path, func(t *testing.T) {
			req := newRouterTestRequest(http.MethodGet, path, testUserToken)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), `"email":"tester@example.com"`)
		})
	}
}

func TestRouter_MerchantProfileUpdateRouteRequiresMerchantRole(t *testing.T) {
	e := newRouterTestEcho()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/merchant/profile", strings.NewReader(`{"store_name":"NomNom Bento"}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+testMerchantToken)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"message":"Merchant profile updated"`)
}

func TestRouter_MerchantProfileUpdateRouteRequiresAuthorization(t *testing.T) {
	e := newRouterTestEcho()

	for _, tc := range []struct {
		name     string
		path     string
		code     int
		token    string
		wantCode string
	}{
		{name: "no token", path: "/api/v1/merchant/profile", token: "", code: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "user role", path: "/api/v1/merchant/profile", token: testUserToken, code: http.StatusForbidden, wantCode: "FORBIDDEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, tc.path, strings.NewReader(`{"store_name":"NomNom Bento"}`))
			if tc.token != "" {
				req.Header.Set(echo.HeaderAuthorization, "Bearer "+tc.token)
			}
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, tc.code, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"`+tc.wantCode+`"`)
		})
	}
}

func TestRouter_AuthenticationRoutesAreRateLimitedPerClientIP(t *testing.T) {
	e := newRouterTestEcho()

	for i := range 30 {
		req := newRouterTestRequest(http.MethodPost, "/auth/login", "")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code, "request %d should be within the burst", i+1)
	}

	req := newRouterTestRequest(http.MethodPost, "/auth/login", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	healthRequest := newRouterTestRequest(http.MethodGet, "/health", "")
	healthResponse := httptest.NewRecorder()
	e.ServeHTTP(healthResponse, healthRequest)
	assert.Equal(t, http.StatusOK, healthResponse.Code)
}

func TestRouter_TestRoutesDisabledByDefault(t *testing.T) {
	e := echo.New()
	r := NewRouter(RouterParams{
		Config: &config.Config{
			TestRoutes: &config.TestRoutesConfig{Enabled: false},
		},
	})
	r.RegisterTestRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test/public", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func newRouterTestEcho() *echo.Echo {
	return newRouterTestEchoWithConfig(&config.Config{})
}

func newRouterTestEchoWithConfig(cfg *config.Config) *echo.Echo {
	userID := uuid.New()
	tokenSvc := &routerTestTokenService{
		claims: map[string]*service.Claims{
			testUserToken: {
				UserID: userID,
				Roles:  []string{string(entity.RoleUser)},
				Type:   service.TokenTypeAccess,
			},
			testMerchantToken: {
				UserID: uuid.New(),
				Roles:  []string{string(entity.RoleMerchant)},
				Type:   service.TokenTypeAccess,
			},
		},
	}

	e := echo.New()
	e.IPExtractor = apimiddleware.NewClientIPExtractor(cfg)
	e.Validator = apivalidator.New()
	authMiddleware := apimiddleware.NewAuthMiddleware(tokenSvc, cfg)
	r := NewRouter(RouterParams{
		UserHandler: handler.NewUserHandler(handler.UserHandlerParams{
			ProfileUC: &routerTestProfileUsecase{},
			Logger:    slog.Default(),
		}),
		DiscoveryHandler: handler.NewDiscoveryHandler(handler.DiscoveryHandlerParams{
			DiscoveryUC: &routerTestDiscoveryUsecase{},
			Logger:      slog.Default(),
		}),
		AuthMiddleware: authMiddleware,
		Config:         cfg,
	})
	if err := r.RegisterRoutes(e); err != nil {
		panic(err)
	}

	return e
}

func newRouterTestRequest(method, target, token string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}

	return req
}
