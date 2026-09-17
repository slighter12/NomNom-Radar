package api

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"radar/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestServeHTTP1AndUnencryptedHTTP2(t *testing.T) {
	t.Parallel()

	echoServer := echo.New()
	echoServer.HideBanner = true
	echoServer.GET("/protocol-probe", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	cfg := &config.Config{}
	srv := &apiServer{
		cfg:    cfg,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		server: echoServer,
	}

	url := startServer(t, srv, cfg)

	t.Run("http1", func(t *testing.T) {
		var protocols http.Protocols
		protocols.SetHTTP1(true)
		resp := getWithProtocols(t, url, &protocols)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, 1, resp.ProtoMajor)
		require.Equal(t, "ok", string(body))
	})

	t.Run("unencrypted_http2", func(t *testing.T) {
		var protocols http.Protocols
		protocols.SetUnencryptedHTTP2(true)
		resp := getWithProtocols(t, url, &protocols)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, 2, resp.ProtoMajor)
		require.Equal(t, "ok", string(body))
	})
}

type protocolServer interface {
	Serve(context.Context) error
	stop(context.Context) error
}

func startServer(t *testing.T, srv protocolServer, cfg *config.Config) string {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	cfg.HTTP.Port = port

	url := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) + "/protocol-probe"

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(context.Background())
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, srv.stop(ctx))

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Serve did not return after shutdown")
		}
	})

	waitUntilReady(t, url)

	return url
}

func waitUntilReady(t *testing.T, url string) {
	t.Helper()

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	client := &http.Client{
		Timeout: 100 * time.Millisecond,
		Transport: &http.Transport{
			Protocols: &protocols,
		},
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			require.NoError(t, resp.Body.Close())

			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("server did not become ready")
}

func getWithProtocols(t *testing.T, url string, protocols *http.Protocols) *http.Response {
	t.Helper()

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Protocols: protocols,
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}
