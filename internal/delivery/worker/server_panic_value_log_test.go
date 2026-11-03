package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"radar/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func panicValueCases() []struct {
	name     string
	value    any
	wantType string
} {
	return []struct {
		name     string
		value    any
		wantType string
	}{
		{
			name:     "map",
			value:    map[string]string{"password": "map-password-secret"},
			wantType: "map[string]string",
		},
		{
			name:     "json-ish error",
			value:    errors.New(`{"password":"plover xylem quokka zither","full_address":"221B Zanzibar Lane"}`),
			wantType: "*errors.errorString",
		},
		{
			name: "echo http error with map message",
			value: echo.NewHTTPError(http.StatusForbidden, map[string]any{
				"password":     "echo-password-secret",
				"full_address": "Evergreen Quixote Terrace",
			}),
			wantType: "*echo.HTTPError",
		},
	}
}

func TestWorkerServer_RecoveredPanicValueNeverReachesLogs(t *testing.T) {
	// Fragments of the sensitive panic values; none may reach any log field.
	secrets := []string{
		"map-password-secret",
		"plover", "xylem", "quokka", "zither",
		"221B", "Zanzibar",
		"echo-password-secret",
		"Evergreen", "Quixote",
	}
	for _, tc := range panicValueCases() {
		t.Run(tc.name, func(t *testing.T) {
			var slogOutput, echoOutput bytes.Buffer
			srv, err := NewServer(ServerParams{
				Lc:     fxtest.NewLifecycle(t),
				Cfg:    &config.Config{},
				Logger: slog.New(slog.NewJSONHandler(&slogOutput, &slog.HandlerOptions{Level: slog.LevelDebug})),
			})
			require.NoError(t, err)
			e := srv.(*workerServer).server
			e.Logger.SetOutput(&echoOutput)
			e.GET("/panic-value", func(echo.Context) error { panic(tc.value) })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic-value", nil))

			assertRecoveredPanicMappedToInternalError(t, rec, slogOutput.String(), "recovered panic ("+tc.wantType+")", "RecoveredPanicValueNeverReachesLogs")
			captured := slogOutput.String() + echoOutput.String() + rec.Body.String()
			for _, secret := range secrets {
				require.NotContains(t, captured, secret)
			}
		})
	}
}
