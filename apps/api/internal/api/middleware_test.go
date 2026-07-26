package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/testutil/fixtures"
)

func TestAuthMiddleware_ValidCredentials(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
}

func TestAuthMiddleware_InvalidCredentials(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "wrong password",
			username: "admin",
			password: "wrongpass",
		},
		{
			name:     "wrong username",
			username: "wronguser",
			password: "secret",
		},
		{
			name:     "both wrong",
			username: "wronguser",
			password: "wrongpass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(tt.username+":"+tt.password)))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Equal(t, `Basic realm="BareD API"`, rr.Header().Get("WWW-Authenticate"))
		})
	}
}

func TestAuthMiddleware_MissingAuth(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, `Basic realm="BareD API"`, rr.Header().Get("WWW-Authenticate"))
}

func TestLoggingMiddleware(t *testing.T) {
	handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test response", rr.Body.String())
}

func TestLoggingMiddleware_CapturesStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "200 OK", statusCode: http.StatusOK},
		{name: "404 Not Found", statusCode: http.StatusNotFound},
		{name: "500 Internal Server Error", statusCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest("GET", "/api/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.statusCode, rr.Code)
		})
	}
}

func TestCorsMiddleware_SameOriginIsEchoed(t *testing.T) {
	server := &Server{}
	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cors enabled"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "dashboard.local:8080"
	req.Header.Set("Origin", "http://dashboard.local:8080")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "http://dashboard.local:8080", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "GET, POST, PUT, PATCH, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", rr.Header().Get("Access-Control-Allow-Headers"))
	assert.Contains(t, rr.Header().Values("Vary"), "Origin")
	assert.Equal(t, "cors enabled", rr.Body.String())
}

// The wildcard is gone: an unknown origin gets no CORS grant at all.
func TestCorsMiddleware_ForeignOriginNotEchoed(t *testing.T) {
	server := &Server{}
	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "dashboard.local:8080"
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, rr.Header().Values("Vary"), "Origin")
}

func TestCorsMiddleware_AllowlistedOrigin(t *testing.T) {
	server := &Server{allowedOrigins: normaliseAllowedOrigins([]string{"http://localhost:5173"})}
	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, "http://localhost:5173", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsMiddleware_PreflightRequest(t *testing.T) {
	server := &Server{}
	handler := server.corsMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("Handler should not be called for OPTIONS request")
	}))

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Host = "dashboard.local:8080"
	req.Header.Set("Origin", "http://dashboard.local:8080")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "http://dashboard.local:8080", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	wrapped.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, wrapped.statusCode)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	// Write without explicitly setting status
	wrapped.Write([]byte("test"))

	assert.Equal(t, http.StatusOK, wrapped.statusCode)
}

func TestRespondSuccess(t *testing.T) {
	rr := httptest.NewRecorder()

	respondSuccess(rr, "operation completed")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), "operation completed")
}

// A server with no credentials configured must not authenticate anyone. The
// previous implementation compared with == against the empty configured values,
// so "Basic <base64 of ':'>" authenticated successfully.
func TestAuthMiddleware_EmptyCredentialsRejected(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := &Server{
		authUser:   "",
		authPass:   "",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":")))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestLoggingMiddleware_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestCorsMiddleware_DifferentOrigins(t *testing.T) {
	server := &Server{}
	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	origins := []string{
		"http://localhost:3000",
		"https://example.com",
		"http://192.168.1.1:8080",
	}

	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Host = "localhost:8080"
			req.Header.Set("Origin", origin)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// None of these is the request host, and none is allowlisted.
			assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func TestAuthMiddleware_CaseInsensitiveUsername(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Try uppercase username
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("ADMIN:secret")))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should fail (case-sensitive)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddlewareChaining(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	// Chain multiple middlewares
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply middlewares in order: cors -> logging -> auth -> handler
	handler := server.corsMiddleware(
		loggingMiddleware(
			server.authMiddleware(finalHandler),
		),
	)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"))
}
