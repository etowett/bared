package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/testutil/fixtures"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	require.NotNil(t, server)
	assert.Equal(t, "localhost:8080", server.addr)
	assert.Equal(t, "admin", server.authUser)
	assert.Equal(t, "secret", server.authPass)
	assert.Equal(t, mgr, server.jobManager)
	assert.Equal(t, cfg, server.cfg)
	assert.Nil(t, server.httpServer) // Not started yet
}

func TestNewServer_NoAuth(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)

	server := NewServer(ServerOptions{Addr: "localhost:8080", JobManager: mgr, Config: cfg})

	require.NotNil(t, server)
	assert.Empty(t, server.authUser)
	assert.Empty(t, server.authPass)
}

func TestSetupRoutes(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	r := server.setupRoutes()

	require.NotNil(t, r)

	// Test that routes are registered by making requests
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		needsAuth      bool
	}{
		{
			name:           "health check no auth",
			method:         "GET",
			path:           "/api/health",
			expectedStatus: http.StatusOK,
			needsAuth:      false,
		},
		{
			name:           "dashboard requires auth",
			method:         "GET",
			path:           "/api/dashboard",
			expectedStatus: http.StatusUnauthorized,
			needsAuth:      true,
		},
		{
			name:           "targets requires auth",
			method:         "GET",
			path:           "/api/targets",
			expectedStatus: http.StatusUnauthorized,
			needsAuth:      true,
		},
		{
			name:           "restore-targets requires auth",
			method:         "GET",
			path:           "/api/restore-targets",
			expectedStatus: http.StatusUnauthorized,
			needsAuth:      true,
		},
		{
			name:           "jobs requires auth",
			method:         "GET",
			path:           "/api/jobs",
			expectedStatus: http.StatusUnauthorized,
			needsAuth:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestSetupRoutes_WithAuth(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	r := server.setupRoutes()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "dashboard with auth",
			method:         "GET",
			path:           "/api/dashboard",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "targets with auth",
			method:         "GET",
			path:           "/api/targets",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "restore-targets with auth",
			method:         "GET",
			path:           "/api/restore-targets",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "jobs with auth",
			method:         "GET",
			path:           "/api/jobs",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.SetBasicAuth("admin", "secret")
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestRouting_Jobs(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	r := server.setupRoutes()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "GET jobs list",
			method:         http.MethodGet,
			path:           "/api/jobs",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET nonexistent job",
			method:         http.MethodGet,
			path:           "/api/jobs/someid",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "DELETE nonexistent job",
			method:         http.MethodDelete,
			path:           "/api/jobs/someid",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "GET job logs",
			method:         http.MethodGet,
			path:           "/api/jobs/someid/logs",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "POST backup no body",
			method:         http.MethodPost,
			path:           "/api/jobs/backup",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST restore no body",
			method:         http.MethodPost,
			path:           "/api/jobs/restore",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "PUT not allowed on jobs list",
			method:         http.MethodPut,
			path:           "/api/jobs",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.SetBasicAuth("admin", "secret")
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestShutdown_NoServer(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	// Should not error when httpServer is nil
	ctx := context.Background()
	err := server.Shutdown(ctx)

	assert.NoError(t, err)
}

func TestShutdown_WithServer(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:0", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg}) // Use port 0 for any available port

	// Create httpServer without actually listening
	r := server.setupRoutes()
	server.httpServer = &http.Server{
		Addr:         server.addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Shutdown should work even though server isn't running
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)

	// Should not error (server wasn't running)
	assert.NoError(t, err)
}

func TestServer_RoutingIntegration(t *testing.T) {
	// Integration test to verify full routing works end-to-end
	target := fixtures.MySQLTarget()
	target.Name = "test-target"

	cfg := &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": fixtures.LocalStorage(),
		},
		Targets: []*config.Target{target},
	}

	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	r := server.setupRoutes()

	tests := []struct {
		name           string
		method         string
		path           string
		auth           bool
		expectedStatus int
	}{
		{
			name:           "health check works",
			method:         "GET",
			path:           "/api/health",
			auth:           false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "dashboard requires auth",
			method:         "GET",
			path:           "/api/dashboard",
			auth:           false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "dashboard works with auth",
			method:         "GET",
			path:           "/api/dashboard",
			auth:           true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "list jobs works with auth",
			method:         "GET",
			path:           "/api/jobs",
			auth:           true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get nonexistent job",
			method:         "GET",
			path:           "/api/jobs/nonexistent",
			auth:           true,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.auth {
				req.SetBasicAuth("admin", "secret")
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestServer_CORSHeaders(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	r := server.setupRoutes()

	// CORS is granted to the request's own origin, never to a wildcard.
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "GET")
}

func TestServer_OptionsRequest(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := NewServer(ServerOptions{Addr: "localhost:8080", AuthUser: "admin", AuthPass: "secret", JobManager: mgr, Config: cfg})

	r := server.setupRoutes()

	// Test OPTIONS (preflight) request
	req := httptest.NewRequest("OPTIONS", "/api/dashboard", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"))
}
