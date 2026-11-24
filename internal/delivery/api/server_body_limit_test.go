package api

import (
	"log/slog"
	"testing"

	"radar/config"
	"radar/internal/delivery/api/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

// The values pin the Echo v4 meaning of existing configuration: decimal units
// are powers of 1000, IEC units powers of 1024.
func TestParseBodyLimit(t *testing.T) {
	t.Parallel()

	valid := map[string]int64{"512": 512, "100KB": 100_000, "1M": 1_000_000, "1KiB": 1024, "1.5KiB": 1536}
	for raw, want := range valid {
		got, err := parseBodyLimit(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}

	for _, raw := range []string{"", "KB", "10XB", "-1K", "99999E", "0", "0KB", "0.5B", "9223372036854775808"} {
		_, err := parseBodyLimit(raw)
		assert.Error(t, err, raw)
	}
}

func TestNewServer_RejectsZeroBodyLimit(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.HTTP.MaxRequestBodySize = "0"

	_, err := NewServer(ServerParams{
		Lc:           fxtest.NewLifecycle(t),
		Cfg:          cfg,
		Logger:       slog.New(slog.DiscardHandler),
		RouterParams: router.RouterParams{Config: cfg},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid http.maxRequestBodySize")
}
