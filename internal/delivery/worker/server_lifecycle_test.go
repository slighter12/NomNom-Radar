package worker

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"radar/config"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

const lifecycleWait = 3 * time.Second

type fetchResult struct {
	status int
	body   string
	err    error
}

// fetch performs a GET and reports every outcome, including transport errors,
// as a value so it is safe to call from a goroutine.
func fetch(url string) fetchResult {
	client := &http.Client{Timeout: lifecycleWait, Transport: &http.Transport{DisableKeepAlives: true}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fetchResult{err: err}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fetchResult{err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	return fetchResult{status: resp.StatusCode, body: string(body), err: err}
}

// stop must let in-flight requests finish (graceful), the same contract Echo
// v4's Shutdown provided. The worker has its own copy of the server wiring, so
// it needs its own check.
func TestWorkerServer_GracefulShutdownCompletesInFlightRequest(t *testing.T) {
	cfg := &config.Config{}
	built, err := NewServer(ServerParams{
		Lc:     fxtest.NewLifecycle(t),
		Cfg:    cfg,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	srv := built.(*workerServer)

	started, release := make(chan struct{}), make(chan struct{})
	var startedOnce, releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	srv.server.GET("/slow", func(c *echo.Context) error {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
		case <-c.Request().Context().Done():
		}

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
	t.Cleanup(func() {
		releaseHandler()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.stop(ctx)
	})

	inFlight := make(chan fetchResult, 1)
	go func() { inFlight <- fetch(baseURL + "/slow") }()
	select {
	case <-started:
	case res := <-inFlight:
		t.Fatalf("request ended before the handler started: %+v", res)
	case <-time.After(lifecycleWait):
		t.Fatal("handler did not start in time")
	}

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

	releaseHandler()
	select {
	case res := <-inFlight:
		require.NoError(t, res.err)
		assert.Equal(t, http.StatusOK, res.status)
		assert.Equal(t, "finished", res.body)
	case <-time.After(lifecycleWait):
		t.Fatal("in-flight request did not complete after release")
	}
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(lifecycleWait):
		t.Fatal("stop did not return after the in-flight request finished")
	}
	select {
	case err := <-served:
		assert.NoError(t, err, "Serve must return cleanly after a graceful stop")
	case <-time.After(lifecycleWait):
		t.Fatal("Serve did not return after stop")
	}
}
