package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"radar/config"
	"radar/internal/domain/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiter_SharesPerIPBucketAcrossAuthGroups(t *testing.T) {
	e := echo.New()
	e.IPExtractor = NewClientIPExtractor(&config.Config{})
	cfg := &config.Config{}
	cfg.HTTP.RateLimit = newRateLimitConfig(1, 2, time.Minute)
	limiter, err := NewAuthRateLimiter(cfg)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	authGroup := e.Group("/auth")
	authGroup.Use(limiter)
	authGroup.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	oauthGroup := e.Group("/oauth")
	oauthGroup.Use(limiter)
	oauthGroup.POST("/google", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for _, path := range []string{"/auth/login", "/oauth/google"} {
		req := newRateLimitRequest(path, "192.0.2.1:1000")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}

	req := newRateLimitRequest("/auth/login", "192.0.2.1:1000")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	otherIPRequest := newRateLimitRequest("/auth/login", "192.0.2.2:1000")
	otherIPResponse := httptest.NewRecorder()
	e.ServeHTTP(otherIPResponse, otherIPRequest)
	assert.Equal(t, http.StatusNoContent, otherIPResponse.Code)
}

func TestAuthRateLimiter_Disabled(t *testing.T) {
	enabled := false
	cfg := &config.Config{}
	cfg.HTTP.RateLimit = &config.RateLimitConfig{Enabled: &enabled}

	limiter, err := NewAuthRateLimiter(cfg)
	require.NoError(t, err)
	assert.Nil(t, limiter)
}

func TestAuthRateLimiter_RejectsInvalidConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.RateLimit = newRateLimitConfig(-1, 1, time.Minute)

	limiter, err := NewAuthRateLimiter(cfg)
	assert.Nil(t, limiter)
	assert.Error(t, err)
}

func TestAPIRateLimiter_UsesSeparateBucketPerUser(t *testing.T) {
	settings := newRateLimitConfig(0.001, 1, time.Minute)
	cfg := &config.Config{}
	cfg.HTTP.APIRateLimit = settings
	limiter, err := NewAPIRateLimiter(cfg)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	e := newUserRateLimitEcho(limiter)
	remoteAddr := "192.0.2.1:1000"
	userA := uuid.New()
	userB := uuid.New()

	userARequest := newUserRateLimitRequest("/api/v1/one", remoteAddr, userA)
	userAResponse := httptest.NewRecorder()
	e.ServeHTTP(userAResponse, userARequest)
	assert.Equal(t, http.StatusNoContent, userAResponse.Code)

	userBRequest := newUserRateLimitRequest("/api/v1/one", remoteAddr, userB)
	userBResponse := httptest.NewRecorder()
	e.ServeHTTP(userBResponse, userBRequest)
	assert.Equal(t, http.StatusNoContent, userBResponse.Code)

	secondUserARequest := newUserRateLimitRequest("/api/v1/one", remoteAddr, userA)
	secondUserAResponse := httptest.NewRecorder()
	e.ServeHTTP(secondUserAResponse, secondUserARequest)
	assert.Equal(t, http.StatusTooManyRequests, secondUserAResponse.Code)
}

func TestAPIRateLimiter_SharesBucketAcrossAPIRoutes(t *testing.T) {
	settings := newRateLimitConfig(0.001, 1, time.Minute)
	cfg := &config.Config{}
	cfg.HTTP.APIRateLimit = settings
	limiter, err := NewAPIRateLimiter(cfg)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	e := newUserRateLimitEcho(limiter)
	userID := uuid.New()
	remoteAddr := "192.0.2.1:1000"

	firstRequest := newUserRateLimitRequest("/api/v1/one", remoteAddr, userID)
	firstResponse := httptest.NewRecorder()
	e.ServeHTTP(firstResponse, firstRequest)
	assert.Equal(t, http.StatusNoContent, firstResponse.Code)

	secondRequest := newUserRateLimitRequest("/api/v1/two", remoteAddr, userID)
	secondResponse := httptest.NewRecorder()
	e.ServeHTTP(secondResponse, secondRequest)
	assert.Equal(t, http.StatusTooManyRequests, secondResponse.Code)
}

func TestAPIRateLimiter_FallsBackToIPWithoutUserID(t *testing.T) {
	settings := newRateLimitConfig(0.001, 1, time.Minute)
	cfg := &config.Config{}
	cfg.HTTP.APIRateLimit = settings
	limiter, err := NewAPIRateLimiter(cfg)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	e := echo.New()
	e.IPExtractor = NewClientIPExtractor(&config.Config{})
	e.Use(limiter)
	e.POST("/api/v1/one", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	firstRequest := newRateLimitRequest("/api/v1/one", "192.0.2.1:1000")
	firstResponse := httptest.NewRecorder()
	e.ServeHTTP(firstResponse, firstRequest)
	assert.Equal(t, http.StatusNoContent, firstResponse.Code)

	secondRequest := newRateLimitRequest("/api/v1/one", "192.0.2.1:1000")
	secondResponse := httptest.NewRecorder()
	e.ServeHTTP(secondResponse, secondRequest)
	assert.Equal(t, http.StatusTooManyRequests, secondResponse.Code)

	otherIPRequest := newRateLimitRequest("/api/v1/one", "192.0.2.2:1000")
	otherIPResponse := httptest.NewRecorder()
	e.ServeHTTP(otherIPResponse, otherIPRequest)
	assert.Equal(t, http.StatusNoContent, otherIPResponse.Code)
}

func TestAPIRateLimiter_Disabled(t *testing.T) {
	enabled := false
	cfg := &config.Config{}
	cfg.HTTP.APIRateLimit = &config.RateLimitConfig{Enabled: &enabled}

	limiter, err := NewAPIRateLimiter(cfg)
	require.NoError(t, err)
	assert.Nil(t, limiter)
}

func TestAPIRateLimiter_RejectsInvalidConfiguration(t *testing.T) {
	settings := newRateLimitConfig(-1, 1, time.Minute)
	cfg := &config.Config{}
	cfg.HTTP.APIRateLimit = settings

	limiter, err := NewAPIRateLimiter(cfg)
	assert.Nil(t, limiter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http.apiRateLimit.rate")
}

func TestSessionRateLimiter_UsesSeparateBucketPerRefreshTokenSubject(t *testing.T) {
	settings := newRateLimitConfig(0.001, 1, time.Minute)
	cfg := &config.Config{}
	cfg.HTTP.SessionRateLimit = settings
	userA := uuid.New()
	userB := uuid.New()
	tokenSvc := &rateLimitTestTokenService{claims: map[string]*service.Claims{
		"refresh-a": {UserID: userA, Type: service.TokenTypeRefresh},
		"refresh-b": {UserID: userB, Type: service.TokenTypeRefresh},
	}}
	e := newSessionRateLimitEcho(t, cfg, tokenSvc)
	remoteAddr := "192.0.2.1:1000"

	firstUserAResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest("refresh-a", remoteAddr))
	assert.Equal(t, http.StatusNoContent, firstUserAResponse.Code)

	firstUserBResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest("refresh-b", remoteAddr))
	assert.Equal(t, http.StatusNoContent, firstUserBResponse.Code)

	secondUserAResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest("refresh-a", remoteAddr))
	assert.Equal(t, http.StatusTooManyRequests, secondUserAResponse.Code)
}

func TestSessionRateLimiter_FallsBackToIPForInvalidOrMissingToken(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		token string
	}{
		{name: "invalid token", token: "invalid-token"},
		{name: "missing token", token: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			settings := newRateLimitConfig(0.001, 1, time.Minute)
			cfg := &config.Config{}
			cfg.HTTP.SessionRateLimit = settings
			e := newSessionRateLimitEcho(t, cfg, &rateLimitTestTokenService{claims: map[string]*service.Claims{}})

			firstResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest(testCase.token, "192.0.2.1:1000"))
			assert.Equal(t, http.StatusNoContent, firstResponse.Code)

			secondResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest(testCase.token, "192.0.2.1:1000"))
			assert.Equal(t, http.StatusTooManyRequests, secondResponse.Code)

			otherIPResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest(testCase.token, "192.0.2.2:1000"))
			assert.Equal(t, http.StatusNoContent, otherIPResponse.Code)
		})
	}
}

func TestSessionRateLimiter_DoesNotAcceptAccessTokenAsSubject(t *testing.T) {
	settings := newRateLimitConfig(0.001, 1, time.Minute)
	cfg := &config.Config{}
	cfg.HTTP.SessionRateLimit = settings
	tokenSvc := &rateLimitTestTokenService{claims: map[string]*service.Claims{
		"access-token": {UserID: uuid.New(), Type: service.TokenTypeAccess},
	}}
	e := newSessionRateLimitEcho(t, cfg, tokenSvc)
	remoteAddr := "192.0.2.1:1000"

	accessTokenResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest("access-token", remoteAddr))
	assert.Equal(t, http.StatusNoContent, accessTokenResponse.Code)

	missingTokenResponse := serveSessionRateLimitRequest(e, newSessionRateLimitRequest("", remoteAddr))
	assert.Equal(t, http.StatusTooManyRequests, missingTokenResponse.Code)
}

func TestSessionRateLimiter_RejectsInvalidConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.SessionRateLimit = newRateLimitConfig(-1, 1, time.Minute)

	limiter, err := NewSessionRateLimiter(cfg, func(echo.Context) (uuid.UUID, bool) {
		return uuid.Nil, false
	})
	assert.Nil(t, limiter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http.sessionRateLimit.rate")
}

func newRateLimitRequest(target, remoteAddr string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, nil)
	req.RemoteAddr = remoteAddr

	return req
}

func newRateLimitConfig(rate float64, burst int, expiresIn time.Duration) *config.RateLimitConfig {
	return &config.RateLimitConfig{
		Rate:      &rate,
		Burst:     &burst,
		ExpiresIn: &expiresIn,
	}
}

func newUserRateLimitEcho(limiter echo.MiddlewareFunc) *echo.Echo {
	e := echo.New()
	e.IPExtractor = NewClientIPExtractor(&config.Config{})
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := uuid.Parse(c.Request().Header.Get("X-Test-User-ID"))
			if err == nil {
				c.Set(string(contextKeyUserID), userID)
			}

			return next(c)
		}
	})
	e.Use(limiter)
	handler := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
	e.POST("/api/v1/one", handler)
	e.POST("/api/v1/two", handler)

	return e
}

func newUserRateLimitRequest(target, remoteAddr string, userID uuid.UUID) *http.Request {
	req := newRateLimitRequest(target, remoteAddr)
	req.Header.Set("X-Test-User-ID", userID.String())

	return req
}

type rateLimitTestTokenService struct {
	claims map[string]*service.Claims
}

func (s *rateLimitTestTokenService) GenerateTokens(uuid.UUID, []string) (string, string, error) {
	return "", "", nil
}

func (s *rateLimitTestTokenService) ValidateToken(token string) (*service.Claims, error) {
	claims, ok := s.claims[token]
	if !ok {
		return nil, assert.AnError
	}

	return claims, nil
}

func (s *rateLimitTestTokenService) GenerateOnboardingToken(uuid.UUID) (string, error) {
	return "", nil
}

func (s *rateLimitTestTokenService) GenerateLinkingToken(uuid.UUID, string, string, string, string) (string, error) {
	return "", nil
}

func (s *rateLimitTestTokenService) GetRefreshTokenDuration() time.Duration {
	return time.Hour
}

func (s *rateLimitTestTokenService) HashToken(token string) string {
	return token
}

func (s *rateLimitTestTokenService) RotateTokens(uuid.UUID, []string) (string, string, string, error) {
	return "", "", "", nil
}

func newSessionRateLimitEcho(t *testing.T, cfg *config.Config, tokenSvc service.TokenService) *echo.Echo {
	t.Helper()
	authMiddleware := NewAuthMiddleware(tokenSvc, cfg)
	limiter, err := NewSessionRateLimiter(cfg, authMiddleware.RefreshTokenSubject)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	e := echo.New()
	e.IPExtractor = NewClientIPExtractor(&config.Config{})
	e.Use(CaptureRequestBodyForErrorLog)
	e.Use(limiter)
	e.POST("/auth/refresh", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	return e
}

func newSessionRateLimitRequest(refreshToken, remoteAddr string) *http.Request {
	body := "{}"
	if refreshToken != "" {
		body = `{"refresh_token":"` + refreshToken + `"}`
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr

	return req
}

func serveSessionRateLimitRequest(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	e.ServeHTTP(response, req)

	return response
}
