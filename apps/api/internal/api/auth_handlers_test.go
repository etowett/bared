package api

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/jobs"
	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

func newAuthTestServer(t *testing.T, opts ...func(*ServerOptions)) *Server {
	t.Helper()

	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	serverOpts := ServerOptions{
		Addr:       "localhost:8080",
		AuthUser:   "admin",
		AuthPass:   "secret",
		JobManager: jobs.NewManager(cfg, nil, nil, 2, 10),
		Config:     cfg,
	}
	for _, o := range opts {
		o(&serverOpts)
	}

	return NewServer(serverOpts)
}

func loginRequestBody(t *testing.T, username, password string) *bytes.Reader {
	t.Helper()

	body, err := json.Marshal(loginRequest{Username: username, Password: password})
	require.NoError(t, err)

	return bytes.NewReader(body)
}

// sessionCookie returns the session cookie from a response, or nil.
func sessionCookie(rr *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

func TestHandleLogin_Success(t *testing.T) {
	server := newAuthTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin", "secret"))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var body identityResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "admin", body.Username)

	cookie := sessionCookie(rr)
	require.NotNil(t, cookie, "login must set a session cookie")
	assert.NotEmpty(t, cookie.Value)
	assert.True(t, cookie.HttpOnly, "cookie must be invisible to JavaScript")
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
	assert.Equal(t, "/", cookie.Path)
	assert.Empty(t, cookie.Domain, "cookie must be host-only")
	assert.Positive(t, cookie.MaxAge)
	assert.False(t, cookie.Expires.IsZero(), "Max-Age and Expires must both be set")

	// The cookie authenticates.
	sess, ok := server.sessions.Validate(cookie.Value)
	require.True(t, ok)
	assert.Equal(t, "admin", sess.username)

	// The password must not be recoverable from the cookie.
	assert.NotContains(t, cookie.Value, "secret")
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	require.NoError(t, err)
	assert.NotContains(t, string(decoded), "secret")
}

// The default deployment is plain HTTP on a LAN; a hard-coded Secure attribute
// would be dropped by the browser and silently break login.
func TestHandleLogin_SecureAttributeIsConditional(t *testing.T) {
	tests := []struct {
		name          string
		secureCookies bool
		tls           bool
		wantSecure    bool
	}{
		{name: "plain http", wantSecure: false},
		{name: "real TLS", tls: true, wantSecure: true},
		{name: "forced for a TLS-terminating proxy", secureCookies: true, wantSecure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newAuthTestServer(t, func(o *ServerOptions) { o.SecureCookies = tt.secureCookies })

			req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin", "secret"))
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			rr := httptest.NewRecorder()

			server.handleLogin(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
			cookie := sessionCookie(rr)
			require.NotNil(t, cookie)
			assert.Equal(t, tt.wantSecure, cookie.Secure)
		})
	}
}

// X-Forwarded-Proto is client-controlled: trusting it would let anyone who can
// reach the daemon directly dictate cookie flags.
func TestHandleLogin_IgnoresForwardedProto(t *testing.T) {
	server := newAuthTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin", "secret"))
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	cookie := sessionCookie(rr)
	require.NotNil(t, cookie)
	assert.False(t, cookie.Secure, "X-Forwarded-Proto must not be trusted")
}

func TestHandleLogin_Failures(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "wrong password", body: `{"username":"admin","password":"wrong"}`, wantStatus: http.StatusUnauthorized},
		{name: "wrong username", body: `{"username":"root","password":"secret"}`, wantStatus: http.StatusUnauthorized},
		{name: "empty credentials", body: `{"username":"","password":""}`, wantStatus: http.StatusUnauthorized},
		{name: "malformed json", body: `{"username":`, wantStatus: http.StatusBadRequest},
		{name: "not json at all", body: `nonsense`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newAuthTestServer(t)

			req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			server.handleLogin(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Nil(t, sessionCookie(rr), "a failed login must not set a session cookie")
			assert.Equal(t, 0, server.sessions.count())
		})
	}
}

// The failure message must not reveal which half of the credential was wrong.
func TestHandleLogin_FailureMessageIsGeneric(t *testing.T) {
	server := newAuthTestServer(t)

	bodies := []string{
		`{"username":"admin","password":"wrong"}`,
		`{"username":"nobody","password":"secret"}`,
	}

	var messages []string
	for _, body := range bodies {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
		rr := httptest.NewRecorder()

		server.handleLogin(rr, req)
		messages = append(messages, rr.Body.String())
	}

	assert.Equal(t, messages[0], messages[1], "wrong-user and wrong-password must be indistinguishable")
}

func TestHandleLogin_OversizedBody(t *testing.T) {
	server := newAuthTestServer(t)

	huge := `{"username":"admin","password":"` + strings.Repeat("a", maxLoginBodyBytes*2) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(huge))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Nil(t, sessionCookie(rr))
}

func TestHandleLogin_NoCredentialsConfigured(t *testing.T) {
	server := newAuthTestServer(t, func(o *ServerOptions) {
		o.AuthUser = ""
		o.AuthPass = ""
	})

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "", ""))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Nil(t, sessionCookie(rr))
}

func TestHandleLogout_RevokesSession(t *testing.T) {
	server := newAuthTestServer(t)

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rr := httptest.NewRecorder()

	server.handleLogout(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	_, ok := server.sessions.Validate(token)
	assert.False(t, ok, "logout must revoke the session server-side, not just clear the cookie")

	cookie := sessionCookie(rr)
	require.NotNil(t, cookie)
	assert.Empty(t, cookie.Value)
	assert.Negative(t, cookie.MaxAge, "logout must expire the cookie")
	assert.Equal(t, "/", cookie.Path, "deletion must match the path the cookie was set with")
}

// Logging out with an already-dead session must still clear the cookie.
func TestHandleLogout_WithoutSession(t *testing.T) {
	server := newAuthTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rr := httptest.NewRecorder()

	server.handleLogout(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	cookie := sessionCookie(rr)
	require.NotNil(t, cookie)
	assert.Negative(t, cookie.MaxAge)
}

func TestHandleMe(t *testing.T) {
	server := newAuthTestServer(t)
	r := server.setupRoutes()

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	t.Run("with a session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var body identityResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "admin", body.Username)
	})

	t.Run("with basic auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.SetBasicAuth("admin", "secret")
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// End-to-end through the router: log in, use the cookie, log out, and find the
// cookie no longer works.
func TestLoginLogoutRoundTrip(t *testing.T) {
	server := newAuthTestServer(t)
	r := server.setupRoutes()

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin", "secret"))
	loginRR := httptest.NewRecorder()
	r.ServeHTTP(loginRR, loginReq)
	require.Equal(t, http.StatusOK, loginRR.Code)

	cookie := sessionCookie(loginRR)
	require.NotNil(t, cookie)

	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(cookie)
	meRR := httptest.NewRecorder()
	r.ServeHTTP(meRR, meReq)
	require.Equal(t, http.StatusOK, meRR.Code)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutRR := httptest.NewRecorder()
	r.ServeHTTP(logoutRR, logoutReq)
	require.Equal(t, http.StatusOK, logoutRR.Code)

	afterReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	afterReq.AddCookie(cookie)
	afterRR := httptest.NewRecorder()
	r.ServeHTTP(afterRR, afterReq)
	assert.Equal(t, http.StatusUnauthorized, afterRR.Code, "the cookie must be dead after logout")
}

func TestCredentialsValid(t *testing.T) {
	server := newAuthTestServer(t)

	assert.True(t, server.credentialsValid("admin", "secret"))
	assert.False(t, server.credentialsValid("admin", "wrong"))
	assert.False(t, server.credentialsValid("Admin", "secret"))
	assert.False(t, server.credentialsValid("", ""))

	noCreds := newAuthTestServer(t, func(o *ServerOptions) {
		o.AuthUser = ""
		o.AuthPass = ""
	})
	assert.False(t, noCreds.credentialsValid("", ""), "an unconfigured server authenticates nobody")
}

func TestSessionTTL_DefaultsWhenUnset(t *testing.T) {
	server := newAuthTestServer(t)
	assert.Equal(t, defaultSessionTTL, server.sessionTTL)

	custom := newAuthTestServer(t, func(o *ServerOptions) { o.SessionTTL = 30 * time.Minute })
	assert.Equal(t, 30*time.Minute, custom.sessionTTL)
}
