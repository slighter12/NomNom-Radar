package router

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"radar/config"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router/handler"
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

func newRouterTestEcho() *echo.Echo {
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
	authMiddleware := apimiddleware.NewAuthMiddleware(tokenSvc, &config.Config{})
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
		Config:         &config.Config{},
	})
	r.RegisterRoutes(e)

	return e
}

func newRouterTestRequest(method, target, token string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}

	return req
}
