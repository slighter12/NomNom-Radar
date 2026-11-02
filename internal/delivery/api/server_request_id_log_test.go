package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router"
	mockService "radar/internal/mocks/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestServer_RejectedRequestIDNeverReachesLifecycleLog(t *testing.T) {
	const rejectedID = "forged id<script>"

	var logs bytes.Buffer
	cfg := &config.Config{}
	cfg.HTTP.MaxRequestBodySize = "1M"
	srv, err := NewServer(ServerParams{
		Lc:     fxtest.NewLifecycle(t),
		Cfg:    cfg,
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		RouterParams: router.RouterParams{
			Config:         cfg,
			AuthMiddleware: apimiddleware.NewAuthMiddleware(mockService.NewMockTokenService(t), cfg),
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/locations/user", nil)
	req.Header.Set("X-Request-Id", rejectedID)
	rec := httptest.NewRecorder()
	srv.(*apiServer).server.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	chosenID := rec.Header().Get("X-Request-Id")
	_, err = uuid.Parse(chosenID)
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &record), logs.String())
	assert.InDelta(t, http.StatusUnauthorized, record["status"], 0)
	assert.Equal(t, chosenID, record["request_id"])
	requestLog, ok := record["request"].(map[string]any)
	require.True(t, ok)
	headersLog, ok := requestLog["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, chosenID, headersLog["X-Request-Id"])
	assert.NotContains(t, logs.String(), "forged")
	assert.NotContains(t, logs.String(), "script")
}
