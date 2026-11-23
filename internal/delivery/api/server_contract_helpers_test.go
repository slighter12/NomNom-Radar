package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"radar/config"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router"
	"radar/internal/delivery/api/router/handler"
	"radar/internal/domain/entity"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

const (
	contractUserToken     = "user-token"
	contractMerchantToken = "merchant-token"
	contractPanicEmail    = "panic@example.com"
)

const (
	contractUserID     = "11111111-1111-1111-1111-111111111111"
	contractMerchantID = "22222222-2222-2222-2222-222222222222"
	contractLocationID = "33333333-3333-3333-3333-333333333333"
)

// contractTokenService accepts two fixed bearer tokens. Unimplemented methods
// are never reached by the contract requests.
type contractTokenService struct{ service.TokenService }

func (contractTokenService) ValidateToken(token string) (*service.Claims, error) {
	switch token {
	case contractUserToken:
		return &service.Claims{UserID: uuid.MustParse(contractUserID), Roles: []string{string(entity.RoleUser)}, Type: service.TokenTypeAccess}, nil
	case contractMerchantToken:
		return &service.Claims{UserID: uuid.MustParse(contractMerchantID), Roles: []string{string(entity.RoleMerchant)}, Type: service.TokenTypeAccess}, nil
	default:
		return nil, errors.New("invalid token")
	}
}

type contractUserUsecase struct{ usecase.UserUsecase }

func (contractUserUsecase) Login(_ context.Context, input *usecase.LoginInput) (*usecase.AuthResult, error) {
	if input.Email == contractPanicEmail {
		panic("contract handler exploded")
	}

	return &usecase.AuthResult{}, nil
}

type contractLocationUsecase struct{ usecase.LocationUsecase }

func (contractLocationUsecase) GetUserLocations(context.Context, uuid.UUID) ([]*entity.Address, error) {
	return []*entity.Address{}, nil
}

func (contractLocationUsecase) UpdateUserLocation(
	context.Context, uuid.UUID, uuid.UUID, *usecase.UpdateLocationInput,
) (*entity.Address, error) {
	return &entity.Address{}, nil
}

type contractDiscoveryUsecase struct{ usecase.DiscoveryUsecase }

func (contractDiscoveryUsecase) SearchPublicMerchants(
	_ context.Context, input *usecase.SearchPublicMerchantsInput,
) (*usecase.SearchPublicMerchantsResult, error) {
	return &usecase.SearchPublicMerchantsResult{
		Merchants:  []*entity.PublicMerchantSearchItem{},
		Pagination: &usecase.MerchantSearchPagination{Page: input.Page, PageSize: input.PageSize},
	}, nil
}

type contractSubscriptionUsecase struct{ usecase.SubscriptionUsecase }

func (contractSubscriptionUsecase) GenerateSubscriptionQR(context.Context, uuid.UUID) ([]byte, error) {
	return []byte("png-bytes"), nil
}

// newContractServer wires the real middleware chain and router with stubbed
// usecases, so tests exercise Echo's behavior exactly as production does.
func newContractServer(t *testing.T, cfg *config.Config) *apiServer {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := NewServer(ServerParams{
		Lc:     fxtest.NewLifecycle(t),
		Cfg:    cfg,
		Logger: logger,
		RouterParams: router.RouterParams{
			Config:         cfg,
			AuthMiddleware: apimiddleware.NewAuthMiddleware(contractTokenService{}, cfg),
			UserHandler: handler.NewUserHandler(handler.UserHandlerParams{
				UserUC: contractUserUsecase{}, Logger: logger,
			}),
			LocationHandler: handler.NewLocationHandler(handler.LocationHandlerParams{
				LocationUC: contractLocationUsecase{}, Logger: logger,
			}),
			DiscoveryHandler: handler.NewDiscoveryHandler(handler.DiscoveryHandlerParams{
				DiscoveryUC: contractDiscoveryUsecase{}, Logger: logger,
			}),
			SubscriptionHandler: handler.NewSubscriptionHandler(handler.SubscriptionHandlerParams{
				SubscriptionUC: contractSubscriptionUsecase{}, Logger: logger,
			}),
		},
	})
	require.NoError(t, err)

	return srv.(*apiServer)
}

func contractConfig(bodyLimit string, burst int) *config.Config {
	rate, expiresIn := 0.001, time.Minute
	cfg := &config.Config{}
	cfg.HTTP.MaxRequestBodySize = bodyLimit
	cfg.HTTP.CORSAllowedOrigins = "https://app.example.com"
	cfg.HTTP.RateLimit = &config.RateLimitConfig{Rate: &rate, Burst: &burst, ExpiresIn: &expiresIn}

	return cfg
}
