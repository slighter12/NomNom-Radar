package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCORSAllowedOrigins(t *testing.T) {
	testCases := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "empty disables CORS", raw: "", want: nil},
		{
			name: "trims and deduplicates origins",
			raw:  " https://app.example.com,https://APP.EXAMPLE.COM, http://localhost:3000 ",
			want: []string{"https://app.example.com", "http://localhost:3000"},
		},
		{name: "rejects wildcard", raw: "*", wantErr: true},
		{name: "rejects path", raw: "https://app.example.com/path", wantErr: true},
		{name: "rejects unsupported scheme", raw: "ftp://app.example.com", wantErr: true},
		{name: "rejects empty list item", raw: "https://app.example.com,", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCORSAllowedOrigins(tc.raw)
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCORSMiddleware_UsesExplicitOrigins(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.CORSAllowedOrigins = "https://app.example.com"
	corsMiddleware, err := NewCORSMiddleware(cfg)
	require.NoError(t, err)
	require.NotNil(t, corsMiddleware)

	e := echo.New()
	e.Use(corsMiddleware)
	e.GET("/resource", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	allowedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/resource", nil)
	allowedRequest.Header.Set(echo.HeaderOrigin, "https://app.example.com")
	allowedResponse := httptest.NewRecorder()
	e.ServeHTTP(allowedResponse, allowedRequest)

	assert.Equal(t, "https://app.example.com", allowedResponse.Header().Get(echo.HeaderAccessControlAllowOrigin))
	assert.NotEqual(t, "*", allowedResponse.Header().Get(echo.HeaderAccessControlAllowOrigin))

	deniedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/resource", nil)
	deniedRequest.Header.Set(echo.HeaderOrigin, "https://evil.example.com")
	deniedResponse := httptest.NewRecorder()
	e.ServeHTTP(deniedResponse, deniedRequest)

	assert.Empty(t, deniedResponse.Header().Get(echo.HeaderAccessControlAllowOrigin))

	preflightRequest := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/resource", nil)
	preflightRequest.Header.Set(echo.HeaderOrigin, "https://app.example.com")
	preflightRequest.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
	preflightRequest.Header.Set(echo.HeaderAccessControlRequestHeaders, echo.HeaderAuthorization+","+echo.HeaderContentType)
	preflightResponse := httptest.NewRecorder()
	e.ServeHTTP(preflightResponse, preflightRequest)

	assert.Equal(t, http.StatusNoContent, preflightResponse.Code)
	assert.Equal(t, "https://app.example.com", preflightResponse.Header().Get(echo.HeaderAccessControlAllowOrigin))
	assert.Contains(t, preflightResponse.Header().Get(echo.HeaderAccessControlAllowHeaders), echo.HeaderAuthorization)
	assert.Empty(t, preflightResponse.Header().Get(echo.HeaderAccessControlAllowCredentials))
}

func TestCORSMiddleware_EmptyConfigurationIsDisabled(t *testing.T) {
	corsMiddleware, err := NewCORSMiddleware(&config.Config{})
	require.NoError(t, err)
	assert.Nil(t, corsMiddleware)
}
