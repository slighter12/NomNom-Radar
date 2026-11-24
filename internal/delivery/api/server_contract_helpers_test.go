package api

import (
	"context"
	"errors"
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
	contractMerchantToken = "merchant-token"
	contractMerchantID    = "22222222-2222-2222-2222-222222222222"
)

// contractTokenService accepts one fixed merchant bearer token. Unimplemented methods
// are never reached by the contract requests.
type contractTokenService struct{ service.TokenService }

func (contractTokenService) ValidateToken(token string) (*service.Claims, error) {
	if token != contractMerchantToken {
		return nil, errors.New("invalid token")
	}

	return &service.Claims{
		UserID: uuid.MustParse(contractMerchantID),
		Roles:  []string{string(entity.RoleMerchant)},
		Type:   service.TokenTypeAccess,
	}, nil
}

type contractUserUsecase struct{ usecase.UserUsecase }

func (contractUserUsecase) Login(context.Context, *usecase.LoginInput) (*usecase.AuthResult, error) {
	return &usecase.AuthResult{}, nil
}

type contractSubscriptionUsecase struct{ usecase.SubscriptionUsecase }

func (contractSubscriptionUsecase) GenerateSubscriptionQR(context.Context, uuid.UUID) ([]byte, error) {
	return []byte("png-bytes"), nil
}

// newContractServer wires the real middleware chain and router with stubbed
// usecases, so tests exercise Echo's behavior exactly as production does.
func newContractServer(t *testing.T, cfg *config.Config) *apiServer {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
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
