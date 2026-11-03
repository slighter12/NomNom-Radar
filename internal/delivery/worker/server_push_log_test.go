package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/config"
	"radar/internal/delivery/worker/handler"
	mockRepo "radar/internal/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

const (
	pushPayloadAddress  = "88 Secret Night Market Rd"
	pushPayloadLocation = "Grandma Stall"
)

func TestWorkerServer_PushFailureLogsOmitPubSubPayload(t *testing.T) {
	for _, tc := range []struct {
		name       string
		repoFails  func(call *mockRepo.MockSubscriptionRepository_FindSubscriberAddressesByUserIDs_Call)
		badEvent   bool
		wantStatus int
	}{
		{
			name:       "malformed event 400",
			badEvent:   true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "retryable 503",
			repoFails: func(call *mockRepo.MockSubscriptionRepository_FindSubscriberAddressesByUserIDs_Call) {
				call.Return(nil, errors.New("database unavailable"))
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "panic 500",
			repoFails: func(call *mockRepo.MockSubscriptionRepository_FindSubscriberAddressesByUserIDs_Call) {
				call.Run(func(context.Context, uuid.UUID, []uuid.UUID) { panic("repository exploded") })
			},
			wantStatus: http.StatusInternalServerError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subscriptionRepo := mockRepo.NewMockSubscriptionRepository(t)
			if tc.repoFails != nil {
				tc.repoFails(subscriptionRepo.EXPECT().FindSubscriberAddressesByUserIDs(mock.Anything, mock.Anything, mock.Anything))
			}

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			cfg := &config.Config{}
			srv, err := NewServer(ServerParams{
				Lc:     fxtest.NewLifecycle(t),
				Cfg:    cfg,
				Logger: logger,
				PushHandler: handler.NewPushHandler(handler.PushHandlerParams{
					Logger:           logger,
					SubscriptionRepo: subscriptionRepo,
				}),
			})
			require.NoError(t, err)

			eventJSON, err := json.Marshal(map[string]any{
				"notification_id": uuid.NewString(),
				"merchant_id":     uuid.NewString(),
				"latitude":        25.0478,
				"longitude":       121.5319,
				"location_name":   pushPayloadLocation,
				"full_address":    pushPayloadAddress,
				"subscriber_ids":  []string{uuid.NewString()},
			})
			require.NoError(t, err)
			if tc.badEvent {
				eventJSON = eventJSON[:len(eventJSON)-1]
			}
			encodedData := base64.StdEncoding.EncodeToString(eventJSON)
			body, err := json.Marshal(map[string]any{
				"message":      map[string]any{"data": encodedData, "messageId": "m-1"},
				"subscription": "projects/p/subscriptions/s",
			})
			require.NoError(t, err)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/push", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.(*workerServer).server.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)

			lifecycle := findLifecycleLogRecord(t, logs.String())
			assert.InDelta(t, tc.wantStatus, lifecycle["status"], 0)
			if requestLog, ok := lifecycle["request"].(map[string]any); ok {
				assert.NotContains(t, requestLog, "body", "lifecycle log must not include the Pub/Sub request body")
			}

			logOutput := logs.String()
			assert.NotContains(t, logOutput, encodedData)
			assert.NotContains(t, logOutput, encodedData[:32])
			assert.NotContains(t, logOutput, pushPayloadAddress)
			assert.NotContains(t, logOutput, pushPayloadLocation)
			assert.NotContains(t, logOutput, "121.5319")
		})
	}
}

func findLifecycleLogRecord(t *testing.T, output string) map[string]any {
	t.Helper()

	var found map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["msg"] == "HTTP request" {
			require.Nil(t, found, "want exactly one lifecycle log record, got:\n%s", output)
			found = record
		}
	}
	require.NotNil(t, found, "no lifecycle log record in:\n%s", output)

	return found
}
