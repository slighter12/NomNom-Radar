package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contractRequest struct {
	method  string
	path    string
	headers map[string]string
	body    string
	chunked bool
	remote  string
}

type contractResponse struct {
	*httptest.ResponseRecorder
}

func (r contractResponse) errorCode(t *testing.T) string {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &body), r.Body.String())

	return body.Error.Code
}

func serveContract(srv *apiServer, c contractRequest) contractResponse {
	req := httptest.NewRequestWithContext(context.Background(), c.method, c.path, strings.NewReader(c.body))
	if c.chunked {
		req.ContentLength = -1
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.remote != "" {
		req.RemoteAddr = c.remote
	}
	rec := httptest.NewRecorder()
	srv.server.ServeHTTP(rec, req)

	return contractResponse{rec}
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func withJSON(headers map[string]string) map[string]string {
	merged := map[string]string{"Content-Type": "application/json"}
	for k, v := range headers {
		merged[k] = v
	}

	return merged
}

// TestServer_HTTPContract pins the Echo v5 behaviors that no other test covers
// and that a handler-level test cannot see: how the assembled server answers
// routing errors, oversized bodies and binary responses. Each row was checked
// against Echo v4.16.5 and v5.4.7; the one intentional deviation is noted.
// Binding, validation, auth, CORS and panic mapping are covered where they
// live (handler, middleware and server_panic tests).
func TestServer_HTTPContract(t *testing.T) {
	t.Parallel()

	loginBody := `{"email":"a@example.com","password":"x"}`
	bigLogin := `{"email":"a@example.com","password":"` + strings.Repeat("a", 2000) + `"}`

	cases := []struct {
		name     string
		req      contractRequest
		status   int
		errCode  string
		wantHdrs map[string]string
	}{
		// Echo v5 routing errors are sentinels, not *echo.HTTPError; the error
		// middleware must still map them to their own 4xx instead of a 500.
		{name: "unmatched route", req: contractRequest{method: "GET", path: "/nope"}, status: 404, errCode: "NOT_FOUND"},
		{name: "wrong method", req: contractRequest{method: "POST", path: "/health"}, status: 405, errCode: "REQUEST_FAILED"},
		// Intentional v5 change: v4 answered 204 for OPTIONS on an unknown path.
		{name: "options on unknown path", req: contractRequest{method: "OPTIONS", path: "/nope"}, status: 404, errCode: "NOT_FOUND"},
		// The two body-limit paths differ in v5: Content-Length is checked up
		// front, a chunked body is only caught while it is being read.
		{
			name: "oversized body by content-length", status: 413, errCode: "REQUEST_FAILED",
			req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: bigLogin},
		},
		{
			name: "oversized chunked body", status: 413, errCode: "REQUEST_FAILED",
			req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: bigLogin, chunked: true},
		},
		{name: "body within limit", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: loginBody}, status: 200},
		{
			name: "binary response keeps its headers", status: 200,
			req: contractRequest{method: "GET", path: "/api/v1/merchant/qr", headers: bearer(contractMerchantToken)},
			wantHdrs: map[string]string{
				"Content-Type":        "image/png",
				"Content-Disposition": "inline; filename=subscription-qr.png",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Each case gets its own server so rate-limit buckets never interact.
			srv := newContractServer(t, contractConfig("1KB", 1000))
			tc.req.remote = "192.0.2.10:1000"
			rec := serveContract(srv, tc.req)

			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.errCode != "" {
				assert.Equal(t, tc.errCode, rec.errorCode(t))
			}
			for header, want := range tc.wantHdrs {
				assert.Equal(t, want, rec.Header().Get(header), header)
			}
		})
	}
}

// Echo v5's memory rate limiter sets X-RateLimit-* and Retry-After itself; v4
// sent neither. Peer-IP keying and forged forwarding headers are covered by
// the rate limiter tests in the middleware package.
func TestServer_RateLimitDeniesWithRetryAfter(t *testing.T) {
	t.Parallel()

	srv := newContractServer(t, contractConfig("1KB", 3))
	login := func() contractResponse {
		return serveContract(srv, contractRequest{
			method: "POST", path: "/auth/login", remote: "198.51.100.7:4000",
			headers: withJSON(nil), body: `{"email":"a@example.com","password":"x"}`,
		})
	}

	for i := range 3 {
		rec := login()
		require.Equal(t, http.StatusOK, rec.Code, "request %d", i)
		assert.Equal(t, "3", rec.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, fmt.Sprint(2-i), rec.Header().Get("X-RateLimit-Remaining"))
	}

	denied := login()
	require.Equal(t, http.StatusTooManyRequests, denied.Code)
	assert.Equal(t, "REQUEST_FAILED", denied.errorCode(t))
	assert.Equal(t, "0", denied.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, denied.Header().Get("Retry-After"))
}

func TestServer_AllowedHostGuardRejectsForeignHostOnErrorPaths(t *testing.T) {
	t.Parallel()

	cfg := contractConfig("1KB", 1000)
	cfg.HTTP.AllowedHost = "api.example.com"
	srv := newContractServer(t, cfg)

	for _, tc := range []struct {
		host   string
		path   string
		status int
	}{
		{host: "api.example.com", path: "/nope", status: http.StatusNotFound},
		{host: "evil.example.com", path: "/nope", status: http.StatusForbidden},
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.path, nil)
		req.Host = tc.host
		rec := httptest.NewRecorder()
		srv.server.ServeHTTP(rec, req)

		assert.Equal(t, tc.status, rec.Code, tc.host)
	}
}
