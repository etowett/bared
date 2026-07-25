package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_AcceptsSessionCookie(t *testing.T) {
	server := newAuthTestServer(t)

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	var gotUser string
	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authFromContext(r.Context())
		require.True(t, ok, "middleware must attach the identity")
		gotUser = auth.username
		assert.NotNil(t, auth.sess, "a cookie-authenticated request carries its session")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "admin", gotUser)
}

// The regression this whole change exists for: the browser cannot set an
// Authorization header on a WebSocket handshake, so a request carrying only the
// session cookie has to authenticate.
func TestAuthMiddleware_WebSocketHandshakeWithCookieOnly(t *testing.T) {
	server := newAuthTestServer(t)

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/abc/logs/stream", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthMiddleware_BasicAuthCarriesNoSession(t *testing.T) {
	server := newAuthTestServer(t)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "admin", auth.username)
		assert.Nil(t, auth.sess, "Basic auth has no session to revoke")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthMiddleware_RejectsBadSessions(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T, s *Server) string
	}{
		{
			name:  "garbage token",
			token: func(_ *testing.T, _ *Server) string { return "not-a-real-token" },
		},
		{
			name: "revoked token",
			token: func(t *testing.T, s *Server) string {
				token, err := s.sessions.Issue("admin")
				require.NoError(t, err)
				s.sessions.Revoke(token)
				return token
			},
		},
		{
			name: "expired token",
			token: func(t *testing.T, s *Server) string {
				now := time.Now()
				s.sessions.now = func() time.Time { return now }
				token, err := s.sessions.Issue("admin")
				require.NoError(t, err)
				now = now.Add(2 * defaultSessionTTL)
				return token
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newAuthTestServer(t)

			handler := server.authMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("handler must not run for an unauthenticated request")
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: tt.token(t, server)})
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)

			// The dead cookie is cleared so the browser stops re-sending it.
			cookie := sessionCookie(rr)
			require.NotNil(t, cookie, "a rejected session cookie should be cleared")
			assert.Negative(t, cookie.MaxAge)
		})
	}
}

// A stale cookie must not lock out a valid Basic-auth client.
func TestAuthMiddleware_BasicAuthRescuesStaleCookie(t *testing.T) {
	server := newAuthTestServer(t)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "stale"})
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// Sending WWW-Authenticate to the dashboard makes a failed XHR pop the
// browser's native credential dialog, which the login form replaces.
func TestAuthMiddleware_WWWAuthenticateOnlyForAPIClients(t *testing.T) {
	tests := []struct {
		name      string
		headers   map[string]string
		cookie    bool
		wantChall bool
	}{
		{name: "curl", wantChall: true},
		{name: "browser fetch", headers: map[string]string{"Sec-Fetch-Mode": "cors"}},
		{name: "browser navigation", headers: map[string]string{"Sec-Fetch-Site": "same-origin"}},
		{name: "stale dashboard session", cookie: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newAuthTestServer(t)

			handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.cookie {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "stale"})
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusUnauthorized, rr.Code)
			if tt.wantChall {
				assert.NotEmpty(t, rr.Header().Get("WWW-Authenticate"))
			} else {
				assert.Empty(t, rr.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestCanonicalOrigin(t *testing.T) {
	tests := []struct {
		origin string
		want   string
	}{
		{origin: "http://localhost:8080", want: "http://localhost:8080"},
		{origin: "http://localhost", want: "http://localhost:80"},
		{origin: "https://example.com", want: "https://example.com:443"},
		{origin: "https://EXAMPLE.com:8443", want: "https://example.com:8443"},
		{origin: "", want: ""},
		{origin: "null", want: ""},
		{origin: "file:///etc/passwd", want: ""},
		{origin: "not a url", want: ""},
		{origin: "http://", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			assert.Equal(t, tt.want, canonicalOrigin(tt.origin))
		})
	}
}

func TestNormaliseAllowedOrigins(t *testing.T) {
	got := normaliseAllowedOrigins([]string{
		"  http://localhost:5173  ",
		"https://dash.example.com",
		"garbage",
		"",
	})

	assert.Equal(t, []string{"http://localhost:5173", "https://dash.example.com:443"}, got)
	assert.Nil(t, normaliseAllowedOrigins(nil))
	assert.Nil(t, normaliseAllowedOrigins([]string{"garbage"}))
}
