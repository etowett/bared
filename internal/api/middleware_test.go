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

func TestBasicAuthMiddleware_ValidCredentials(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.basicAuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
}

func TestBasicAuthMiddleware_InvalidCredentials(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)

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
			handler := server.basicAuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(tt.username+":"+tt.password)))
			rr := httptest.NewRecorder()

			handler(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Equal(t, `Basic realm="BareD API"`, rr.Header().Get("WWW-Authenticate"))
		})
	}
}

func TestBasicAuthMiddleware_MissingAuth(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.basicAuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, `Basic realm="BareD API"`, rr.Header().Get("WWW-Authenticate"))
}

func TestLoggingMiddleware(t *testing.T) {
	handler := loggingMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

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
			handler := loggingMiddleware(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			req := httptest.NewRequest("GET", "/api/test", nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			assert.Equal(t, tt.statusCode, rr.Code)
		})
	}
}

func TestCorsMiddleware(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cors enabled"))
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", rr.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "cors enabled", rr.Body.String())
}

func TestCorsMiddleware_PreflightRequest(t *testing.T) {
	handler := corsMiddleware(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("Handler should not be called for OPTIONS request")
	})

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
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

func TestBasicAuthMiddleware_EmptyCredentials(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)

	server := &Server{
		authUser:   "",
		authPass:   "",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.basicAuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":")))
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestLoggingMiddleware_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := loggingMiddleware(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestCorsMiddleware_DifferentOrigins(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	origins := []string{
		"http://localhost:3000",
		"https://example.com",
		"http://192.168.1.1:8080",
	}

	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Origin", origin)
			rr := httptest.NewRecorder()

			handler(rr, req)

			// Should allow all origins (*)
			assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func TestBasicAuthMiddleware_CaseInsensitiveUsername(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	handler := server.basicAuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Try uppercase username
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("ADMIN:secret")))
	rr := httptest.NewRecorder()

	handler(rr, req)

	// Should fail (case-sensitive)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddlewareChaining(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)

	server := &Server{
		authUser:   "admin",
		authPass:   "secret",
		jobManager: mgr,
		cfg:        cfg,
	}

	// Chain multiple middlewares
	finalHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}

	// Apply middlewares in order: cors -> logging -> auth -> handler
	handler := corsMiddleware(
		loggingMiddleware(
			server.basicAuthMiddleware(finalHandler),
		),
	)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}
