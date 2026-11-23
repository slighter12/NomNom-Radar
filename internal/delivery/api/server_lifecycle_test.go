package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"radar/config"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func protocolClient(protocols *http.Protocols) *http.Client {
	return &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{Protocols: protocols, DisableKeepAlives: true},
	}
}

type observedResponse struct {
	StatusCode int
	ProtoMajor int
	RequestID  string
}

func doRequest(t *testing.T, client *http.Client, method, url, body string, headers map[string]string) (observedResponse, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	return observedResponse{
		StatusCode: resp.StatusCode,
		ProtoMajor: resp.ProtoMajor,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}, string(payload)
}

// The full middleware chain must behave the same over a real socket as it does
// under httptest: real peer address, real Content-Length handling, h1 and h2c.
func TestServer_FullStackOverRealTCP(t *testing.T) {
	cfg := contractConfig("1KB", 2)
	srv := newContractServer(t, cfg)
	baseURL, _ := startContractServer(t, srv, cfg)

	var h1, h2c http.Protocols
	h1.SetHTTP1(true)
	h2c.SetUnencryptedHTTP2(true)

	for name, protocols := range map[string]*http.Protocols{"http1": &h1, "h2c": &h2c} {
		t.Run(name, func(t *testing.T) {
			client := protocolClient(protocols)
			wantMajor := map[string]int{"http1": 1, "h2c": 2}[name]

			resp, body := doRequest(t, client, http.MethodGet, baseURL+"/health", "", nil)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, wantMajor, resp.ProtoMajor)
			assert.NotEmpty(t, resp.RequestID)
			assert.Contains(t, body, "ok")

			resp, body = doRequest(t, client, http.MethodGet, baseURL+"/nope", "", nil)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.Contains(t, body, `"code":"NOT_FOUND"`)

			resp, body = doRequest(t, client, http.MethodPut, baseURL+"/api/v1/locations/user/not-a-uuid",
				`{}`, withJSON(bearer(contractUserToken)))
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, body, `"code":"INVALID_ID"`)

			resp, _ = doRequest(t, client, http.MethodPost, baseURL+"/auth/login",
				`{"email":"a@example.com","password":"`+strings.Repeat("a", 2000)+`"}`, withJSON(nil))
			assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
		})
	}

	t.Run("rate limit keys on the real peer, not forged headers", func(t *testing.T) {
		client := protocolClient(&h1)
		statuses := make([]int, 0, 4)
		for i := range 4 {
			resp, _ := doRequest(t, client, http.MethodPost, baseURL+"/oauth/google/callback", `{}`,
				withJSON(map[string]string{"X-Forwarded-For": "203.0.113." + strconv.Itoa(i)}))
			statuses = append(statuses, resp.StatusCode)
		}

		assert.NotContains(t, statuses[:2], http.StatusTooManyRequests, "burst of 2 is allowed")
		assert.Equal(t, http.StatusTooManyRequests, statuses[3], "forged X-Forwarded-For must not mint a new bucket")
	})
}

// stop must let in-flight requests finish (graceful) and refuse new ones, the
// same contract Echo v4's Shutdown provided.
func TestServer_GracefulShutdownCompletesInFlightRequest(t *testing.T) {
	cfg := contractConfig("1KB", 1000)
	srv := newContractServer(t, cfg)
	started, release := make(chan struct{}), make(chan struct{})
	srv.server.GET("/slow", func(c *echo.Context) error {
		close(started)
		<-release

		return c.String(http.StatusOK, "finished")
	})
	baseURL, served := startContractServer(t, srv, cfg)

	type result struct {
		status int
		body   string
	}
	inFlight := make(chan result, 1)
	go func() {
		resp, body := doRequest(t, protocolClient(&http.Protocols{}), http.MethodGet, baseURL+"/slow", "", nil)
		inFlight <- result{resp.StatusCode, body}
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
	assert.Eventually(t, func() bool {
		dialer := &net.Dialer{Timeout: 100 * time.Millisecond}
		conn, err := dialer.DialContext(context.Background(), "tcp", strings.TrimPrefix(baseURL, "http://"))
		if err == nil {
			_ = conn.Close()
		}

		return err != nil
	}, time.Second, 20*time.Millisecond, "listener should stop accepting once shutdown begins")

	close(release)
	assert.Equal(t, result{http.StatusOK, "finished"}, <-inFlight)
	require.NoError(t, <-stopped)
	assert.NoError(t, (<-served).err, "Serve must return cleanly after a graceful stop")
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
