package worker

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"radar/config"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func newLifecycleWorker(t *testing.T) (*workerServer, *config.Config) {
	t.Helper()

	cfg := &config.Config{}
	srv, err := NewServer(ServerParams{
		Lc:     fxtest.NewLifecycle(t),
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)

	return srv.(*workerServer), cfg
}

func TestWorkerServer_RoutingErrorsKeepTheirStatus(t *testing.T) {
	t.Parallel()

	srv, _ := newLifecycleWorker(t)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/health", http.StatusOK},
	} {
		req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.server.ServeHTTP(rec, req)

		assert.Equal(t, tc.want, rec.Code, tc.method+" "+tc.path)
	}
}

func TestWorkerServer_GracefulShutdownCompletesInFlightRequest(t *testing.T) {
	srv, cfg := newLifecycleWorker(t)
	started, release := make(chan struct{}), make(chan struct{})
	srv.server.GET("/slow", func(c *echo.Context) error {
		close(started)
		<-release

		return c.String(http.StatusOK, "finished")
	})

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "0.0.0.0:0")
	require.NoError(t, err)
	cfg.HTTP.Port = ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	baseURL := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.HTTP.Port))

	served := make(chan error, 1)
	go func() { served <- srv.Serve(context.Background()) }()
	waitUntilReady(t, baseURL+"/health")

	inFlight := make(chan string, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/slow", nil)
		resp, err := client.Do(req)
		if err != nil {
			inFlight <- "error: " + err.Error()

			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		inFlight <- strconv.Itoa(resp.StatusCode) + " " + string(body)
	}()
	<-started

	stopped := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopped <- srv.stop(ctx)
	}()
	select {
	case err := <-stopped:
		t.Fatalf("stop returned (%v) while a request was still in flight", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(release)
	assert.Equal(t, "200 finished", <-inFlight)
	require.NoError(t, <-stopped)
	assert.NoError(t, <-served)
}

func TestWorkerServer_ServeFailsWhenPortIsTaken(t *testing.T) {
	t.Parallel()

	occupied, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "0.0.0.0:0")
	require.NoError(t, err)
	defer occupied.Close()

	srv, cfg := newLifecycleWorker(t)
	cfg.HTTP.Port = occupied.Addr().(*net.TCPAddr).Port

	err = srv.Serve(context.Background())

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "listen on"), err.Error())
}
