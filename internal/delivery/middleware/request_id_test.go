package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"radar/internal/platform/observability"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestIDResult struct {
	header  string
	context string
	logged  string
}

func processRequestID(t *testing.T, header *string) requestIDResult {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if header != nil {
		req.Header[observability.HeaderXRequestID] = []string{*header}
	}
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	var logs bytes.Buffer
	var contextID string
	handler := NewRequestIDMiddleware(slog.New(slog.NewJSONHandler(&logs, nil))).Process(func(c echo.Context) error {
		ctx := c.Request().Context()
		contextID = observability.CorrelationIDFromContext(ctx)
		observability.LoggerFromContext(ctx).InfoContext(ctx, "probe")

		return nil
	})
	require.NoError(t, handler(c))

	var record struct {
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(logs.Bytes(), &record))

	return requestIDResult{
		header:  rec.Header().Get(observability.HeaderXRequestID),
		context: contextID,
		logged:  record.RequestID,
	}
}

func TestRequestIDMiddleware_PreservesValidClientID(t *testing.T) {
	for _, id := range []string{"abc-123_X.y", strings.Repeat("a", 128)} {
		got := processRequestID(t, &id)

		assert.Equal(t, requestIDResult{header: id, context: id, logged: id}, got)
	}
}

func TestRequestIDMiddleware_ReplacesInvalidClientID(t *testing.T) {
	for name, id := range map[string]string{
		"too long":          strings.Repeat("a", 129),
		"space":             "abc 123",
		"control character": "abc\x01123",
		"angle bracket":     "abc<123",
		"empty":             "",
	} {
		t.Run(name, func(t *testing.T) {
			got := processRequestID(t, &id)

			_, err := uuid.Parse(got.header)
			require.NoError(t, err, "header ID %q is not a UUID", got.header)
			assert.Equal(t, requestIDResult{header: got.header, context: got.header, logged: got.header}, got)
		})
	}
}

func TestRequestIDMiddleware_GeneratesIDWhenHeaderMissing(t *testing.T) {
	got := processRequestID(t, nil)

	_, err := uuid.Parse(got.header)
	require.NoError(t, err)
	assert.Equal(t, requestIDResult{header: got.header, context: got.header, logged: got.header}, got)
}
