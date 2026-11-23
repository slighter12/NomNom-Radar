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

func preflight(origin, method string) map[string]string {
	return map[string]string{
		"Origin":                         origin,
		"Access-Control-Request-Method":  method,
		"Access-Control-Request-Headers": "authorization,content-type",
	}
}

// TestServer_HTTPContract pins the externally visible HTTP behavior of the
// full middleware chain and router. Every row below produced the same status
// and error code on Echo v4.16.5 and Echo v5.4.7 (verified by running the same
// requests against both versions); the only intentional deviation is noted.
func TestServer_HTTPContract(t *testing.T) {
	t.Parallel()

	const oversized = 2000
	loginBody := `{"email":"a@example.com","password":"x"}`
	bigLogin := `{"email":"a@example.com","password":"` + strings.Repeat("a", oversized) + `"}`
	locationPath := "/api/v1/locations/user/" + contractLocationID

	cases := []struct {
		name     string
		req      contractRequest
		status   int
		errCode  string
		wantHdrs map[string]string
	}{
		{name: "unmatched route", req: contractRequest{method: "GET", path: "/nope"}, status: 404, errCode: "NOT_FOUND"},
		{name: "wrong method", req: contractRequest{method: "POST", path: "/health"}, status: 405, errCode: "REQUEST_FAILED"},
		{name: "trailing slash is not routed", req: contractRequest{method: "GET", path: "/health/"}, status: 404, errCode: "NOT_FOUND"},
		// Intentional v5 change: v4 answered 204 for OPTIONS on an unknown path.
		{name: "options on unknown path", req: contractRequest{method: "OPTIONS", path: "/nope"}, status: 404, errCode: "NOT_FOUND"},
		{
			name: "preflight from allowed origin", status: 204,
			req:      contractRequest{method: "OPTIONS", path: "/auth/login", headers: preflight("https://app.example.com", "POST")},
			wantHdrs: map[string]string{"Access-Control-Allow-Origin": "https://app.example.com"},
		},
		{
			name: "preflight from foreign origin", status: 204,
			req:      contractRequest{method: "OPTIONS", path: "/auth/login", headers: preflight("https://evil.example.com", "POST")},
			wantHdrs: map[string]string{"Access-Control-Allow-Origin": ""},
		},
		{
			name: "simple request from allowed origin", status: 200,
			req:      contractRequest{method: "GET", path: "/health", headers: map[string]string{"Origin": "https://app.example.com"}},
			wantHdrs: map[string]string{"Access-Control-Allow-Origin": "https://app.example.com"},
		},
		{
			name: "simple request from foreign origin", status: 200,
			req:      contractRequest{method: "GET", path: "/health", headers: map[string]string{"Origin": "https://evil.example.com"}},
			wantHdrs: map[string]string{"Access-Control-Allow-Origin": ""},
		},
		{name: "malformed json", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: `{"email":`}, status: 400, errCode: "INVALID_INPUT"},
		{name: "wrong content type", req: contractRequest{method: "POST", path: "/auth/login", headers: map[string]string{"Content-Type": "text/plain"}, body: loginBody}, status: 400, errCode: "INVALID_INPUT"},
		{name: "json type mismatch", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: `{"email":1,"password":"x"}`}, status: 400, errCode: "INVALID_INPUT"},
		{name: "empty body fails validation", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil)}, status: 400, errCode: "VALIDATION_FAILED"},
		{name: "null body fails validation", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: `null`}, status: 400, errCode: "VALIDATION_FAILED"},
		{name: "field validation", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: `{"email":"nope"}`}, status: 400, errCode: "VALIDATION_FAILED"},
		{name: "valid login", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: loginBody}, status: 200},
		{name: "oversized body by content-length", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: bigLogin}, status: 413, errCode: "REQUEST_FAILED"},
		{name: "oversized chunked body", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: bigLogin, chunked: true}, status: 413, errCode: "REQUEST_FAILED"},
		{name: "handler panic", req: contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: `{"email":"` + contractPanicEmail + `","password":"x"}`}, status: 500, errCode: "INTERNAL_ERROR"},
		{name: "missing token", req: contractRequest{method: "GET", path: "/api/v1/user/profile"}, status: 401, errCode: "UNAUTHORIZED"},
		{name: "unknown token", req: contractRequest{method: "GET", path: "/api/v1/user/profile", headers: bearer("zzz")}, status: 401, errCode: "INVALID_TOKEN"},
		{name: "non-bearer scheme", req: contractRequest{method: "GET", path: "/api/v1/user/profile", headers: map[string]string{"Authorization": "Basic abc"}}, status: 401, errCode: "INVALID_TOKEN"},
		{name: "wrong role", req: contractRequest{method: "GET", path: "/api/v1/locations/user", headers: bearer(contractMerchantToken)}, status: 403, errCode: "FORBIDDEN"},
		{name: "authorized list", req: contractRequest{method: "GET", path: "/api/v1/locations/user", headers: bearer(contractUserToken)}, status: 200},
		{name: "path param not a uuid", req: contractRequest{method: "PUT", path: "/api/v1/locations/user/not-a-uuid", headers: withJSON(bearer(contractUserToken)), body: `{}`}, status: 400, errCode: "INVALID_ID"},
		{name: "path param with encoded slash", req: contractRequest{method: "PUT", path: locationPath + "%2Fx", headers: withJSON(bearer(contractUserToken)), body: `{}`}, status: 400, errCode: "INVALID_ID"},
		{name: "path param double encoded", req: contractRequest{method: "PUT", path: "/api/v1/locations/user/%2533333333-3333-3333-3333-333333333333", headers: withJSON(bearer(contractUserToken)), body: `{}`}, status: 400, errCode: "INVALID_ID"},
		{name: "path param valid", req: contractRequest{method: "PUT", path: locationPath, headers: withJSON(bearer(contractUserToken)), body: `{"label":"home"}`}, status: 200},
		{name: "body validation on update", req: contractRequest{method: "PUT", path: locationPath, headers: withJSON(bearer(contractUserToken)), body: `{"latitude":999}`}, status: 400, errCode: "VALIDATION_FAILED"},
		{name: "query binding ok", req: contractRequest{method: "GET", path: "/api/v1/merchants?page=2&page_size=5", headers: bearer(contractUserToken)}, status: 200},
		{name: "query binding bad int", req: contractRequest{method: "GET", path: "/api/v1/merchants?page=abc", headers: bearer(contractUserToken)}, status: 400, errCode: "INVALID_INPUT"},
		{name: "query binding bad float", req: contractRequest{method: "GET", path: "/api/v1/merchants?latitude=x", headers: bearer(contractUserToken)}, status: 400, errCode: "INVALID_INPUT"},
		{name: "query validation", req: contractRequest{method: "GET", path: "/api/v1/merchants?latitude=999", headers: bearer(contractUserToken)}, status: 400, errCode: "VALIDATION_FAILED"},
		{name: "query uuid validation", req: contractRequest{method: "GET", path: "/api/v1/merchants?category_id=zzz", headers: bearer(contractUserToken)}, status: 400, errCode: "VALIDATION_FAILED"},
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

func TestServer_RequestIDIsEchoedOnRoutingErrors(t *testing.T) {
	t.Parallel()

	srv := newContractServer(t, contractConfig("1KB", 1000))
	rec := serveContract(srv, contractRequest{method: "GET", path: "/nope", headers: map[string]string{"X-Request-Id": "abc-123"}})

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "abc-123", rec.Header().Get("X-Request-Id"))
	assert.Contains(t, rec.Body.String(), `"request_id":"abc-123"`)
}

func TestServer_BodyAtLimitIsAccepted(t *testing.T) {
	t.Parallel()

	srv := newContractServer(t, contractConfig("1KB", 1000))
	padding := 1000 - len(`{"email":"a@example.com","password":""}`)
	body := `{"email":"a@example.com","password":"` + strings.Repeat("a", padding) + `"}`
	require.Len(t, body, 1000) // "1KB" is 1000 bytes, as it was under Echo v4

	rec := serveContract(srv, contractRequest{method: "POST", path: "/auth/login", headers: withJSON(nil), body: body, remote: "192.0.2.10:1"})

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Echo v5's memory rate limiter sets X-RateLimit-* and Retry-After itself; v4
// sent neither. The limiter must still key on the network peer, never on
// forwarded headers a client can forge.
func TestServer_RateLimitDeniesWithRetryAfterAndIgnoresForgedForwarding(t *testing.T) {
	t.Parallel()

	srv := newContractServer(t, contractConfig("1KB", 3))
	login := func(forwardedFor, remote string) contractResponse {
		return serveContract(srv, contractRequest{
			method: "POST", path: "/auth/login", remote: remote,
			headers: withJSON(map[string]string{"X-Forwarded-For": forwardedFor}),
			body:    `{"email":"a@example.com","password":"x"}`,
		})
	}

	for i := range 3 {
		rec := login(fmt.Sprintf("203.0.113.%d", i), "198.51.100.7:4000")
		require.Equal(t, http.StatusOK, rec.Code, "request %d", i)
		assert.Equal(t, "3", rec.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, fmt.Sprint(2-i), rec.Header().Get("X-RateLimit-Remaining"))
	}

	denied := login("203.0.113.99", "198.51.100.7:4000")
	require.Equal(t, http.StatusTooManyRequests, denied.Code)
	assert.Equal(t, "REQUEST_FAILED", denied.errorCode(t))
	assert.Equal(t, "0", denied.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, denied.Header().Get("Retry-After"))

	otherPeer := login("203.0.113.99", "198.51.100.8:4000")
	assert.Equal(t, http.StatusOK, otherPeer.Code)
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
