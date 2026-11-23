package api

import (
	"io"
	"log/slog"
	"testing"

	"radar/config"
	"radar/internal/delivery/api/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

// Expected values were produced by labstack/gommon/bytes.Parse, the parser
// Echo v4's BodyLimit used, so existing configuration keeps its meaning.
func TestParseBodyLimit(t *testing.T) {
	t.Parallel()

	valid := map[string]int64{
		"512": 512, "1B": 1, "1K": 1000, "1KB": 1000, "1kb": 1000, "1 KB": 1000, "100KB": 100_000,
		"1M": 1_000_000, "1MB": 1_000_000, "1.5K": 1500, "2G": 2_000_000_000, "1T": 1_000_000_000_000,
		"1KiB": 1024, "1Ki": 1024, "1MiB": 1 << 20, "2 GiB": 2 << 30, "1.5KiB": 1536,
	}
	for raw, want := range valid {
		got, err := parseBodyLimit(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}

	invalid := []string{
		"", "KB", "10XB", "-1K", "1..5M", " 1K", "1K ", "99999E", // malformed or overflowing
		"0", "0B", "0KB", "0.5B", // zero is not a usable limit (see parseBodyLimit)
	}
	for _, raw := range invalid {
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
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		RouterParams: router.RouterParams{Config: cfg},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be greater than zero")
}
