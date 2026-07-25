package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CSRF is exercised against real state-changing endpoints rather than the
// middleware alone, so a routing mistake that skips it cannot pass.
func TestCSRF_StateChangingEndpoints(t *testing.T) {
	endpoints := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "trigger backup", method: http.MethodPost, path: "/api/jobs/backup", body: `{"target":"test-mysql"}`},
		{name: "trigger restore", method: http.MethodPost, path: "/api/jobs/restore", body: `{"target":"test-mysql"}`},
		{name: "cancel job", method: http.MethodDelete, path: "/api/jobs/some-id", body: ""},
	}

	origins := []struct {
		name       string
		origin     string
		wantReject bool
	}{
		{name: "same origin", origin: "http://localhost:8080"},
		{name: "allowlisted dev server", origin: "http://localhost:5173"},
		{name: "foreign origin", origin: "https://evil.example", wantReject: true},
		{name: "lookalike host", origin: "http://evil-localhost:8080", wantReject: true},
		{name: "no origin at all", origin: "", wantReject: true},
	}

	for _, ep := range endpoints {
		for _, o := range origins {
			t.Run(ep.name+"/"+o.name, func(t *testing.T) {
				server := newAuthTestServer(t, func(opts *ServerOptions) {
					opts.AllowedOrigins = []string{"http://localhost:5173"}
				})
				r := server.setupRoutes()

				token, err := server.sessions.Issue("admin")
				require.NoError(t, err)

				req := httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
				req.Host = "localhost:8080"
				req.Header.Set("Content-Type", "application/json")
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
				if o.origin != "" {
					req.Header.Set("Origin", o.origin)
				}
				rr := httptest.NewRecorder()

				r.ServeHTTP(rr, req)

				if o.wantReject {
					assert.Equal(t, http.StatusForbidden, rr.Code,
						"cookie-authenticated %s from %q must be rejected", ep.method, o.origin)
					return
				}
				// Anything but 403 means CSRF let it through to the handler,
				// which may still fail for unrelated reasons (unknown job etc).
				assert.NotEqual(t, http.StatusForbidden, rr.Code)
			})
		}
	}
}

// Basic-auth clients send no ambient credential, so a hostile page cannot forge
// their requests and they must not be subject to the origin check.
func TestCSRF_BasicAuthIsExempt(t *testing.T) {
	server := newAuthTestServer(t)
	r := server.setupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/backup", strings.NewReader(`{"target":"test-mysql"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusForbidden, rr.Code)
}

// Reads are outside the CSRF threat model — a forged GET cannot be read
// cross-origin without a CORS grant, which foreign origins do not get.
func TestCSRF_SafeMethodsAreNotChecked(t *testing.T) {
	server := newAuthTestServer(t)
	r := server.setupRoutes()

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestIsSafeMethod(t *testing.T) {
	safe := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	unsafe := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for _, m := range safe {
		assert.True(t, isSafeMethod(m), m)
	}
	for _, m := range unsafe {
		assert.False(t, isSafeMethod(m), m)
	}
}
