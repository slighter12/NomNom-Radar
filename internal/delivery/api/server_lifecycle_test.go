package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"radar/config"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lifecycleWait = 3 * time.Second

type serveResult struct{ err error }

// startContractServer serves the full stack on a real loopback port and returns
// its base URL. The server is shut down when the test ends.
func startContractServer(t *testing.T, srv *apiServer, cfg *config.Config) (baseURL string, served <-chan serveResult) {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "0.0.0.0:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	cfg.HTTP.Port = port

	ch := make(chan serveResult, 1)
	go func() { ch <- serveResult{err: srv.Serve(context.Background())} }()
	baseURL = "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	waitUntilReady(t, baseURL+"/health")

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.stop(ctx)
	})

	return baseURL, ch
}

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

// stop must let in-flight requests finish (graceful) and refuse new ones, the
// same contract Echo v4's Shutdown provided.
func TestServer_GracefulShutdownCompletesInFlightRequest(t *testing.T) {
	cfg := contractConfig("1KB", 1000)
	srv := newContractServer(t, cfg)

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
	baseURL, served := startContractServer(t, srv, cfg)
	// Registered after startContractServer's cleanup, so it runs first and a
	// failed assertion can never leave the handler (and the stop) blocked.
	t.Cleanup(releaseHandler)

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
	assert.Eventually(t, func() bool {
		dialer := &net.Dialer{Timeout: 100 * time.Millisecond}
		conn, err := dialer.DialContext(context.Background(), "tcp", strings.TrimPrefix(baseURL, "http://"))
		if err == nil {
			_ = conn.Close()
		}

		return err != nil
	}, time.Second, 20*time.Millisecond, "listener should stop accepting once shutdown begins")

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
	case res := <-served:
		assert.NoError(t, res.err, "Serve must return cleanly after a graceful stop")
	case <-time.After(lifecycleWait):
		t.Fatal("Serve did not return after stop")
	}
}

func TestServer_ServeFailsWhenPortIsTaken(t *testing.T) {
	t.Parallel()

	occupied, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "0.0.0.0:0")
	require.NoError(t, err)
	defer occupied.Close()

	cfg := contractConfig("1KB", 1000)
	cfg.HTTP.Port = occupied.Addr().(*net.TCPAddr).Port
	srv := newContractServer(t, cfg)

	err = srv.Serve(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listen on")
}
