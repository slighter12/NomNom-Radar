package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/internal/platform/observability"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const contextRequestID = "ctx-req-1"

func handlePushAndCaptureRequestID(t *testing.T, attributeID, eventID string) string {
	t.Helper()

	eventJSON, err := json.Marshal(map[string]string{"request_id": eventID})
	require.NoError(t, err)
	pushMsg := PubSubMessage{}
	pushMsg.Message.Data = base64.StdEncoding.EncodeToString(eventJSON)
	if attributeID != "" {
		pushMsg.Message.Attributes = map[string]string{"request_id": attributeID}
	}
	body, err := json.Marshal(pushMsg)
	require.NoError(t, err)

	ctx := observability.WithCorrelationID(context.Background(), contextRequestID)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/push", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	var logs bytes.Buffer
	h := NewPushHandler(PushHandlerParams{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	require.NoError(t, h.HandlePush(echo.New().NewContext(req, rec)))

	firstLine, _, _ := strings.Cut(logs.String(), "\n")
	var record struct {
		Msg       string `json:"msg"`
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(firstLine), &record))
	require.Equal(t, "[Worker] Processing notification event", record.Msg)

	return record.RequestID
}

func TestPushHandler_KeepsValidAttributeRequestID(t *testing.T) {
	assert.Equal(t, "attr-req-1", handlePushAndCaptureRequestID(t, "attr-req-1", "event-req-1"))
}

func TestPushHandler_MalformedAttributeRequestIDFallsThroughToEvent(t *testing.T) {
	for name, attributeID := range map[string]string{
		"space":         "attr req",
		"angle bracket": "attr<script>",
		"too long":      strings.Repeat("a", 129),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "event-req-1", handlePushAndCaptureRequestID(t, attributeID, "event-req-1"))
		})
	}
}

func TestPushHandler_MalformedAttributeAndEventRequestIDsFallThroughToContext(t *testing.T) {
	assert.Equal(t, contextRequestID, handlePushAndCaptureRequestID(t, "attr\nforged", strings.Repeat("e", 129)))
}
