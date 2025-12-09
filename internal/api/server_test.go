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
	mgr := jobs.NewManager(cfg, 2, 10)

	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

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
	mgr := jobs.NewManager(cfg, 2, 10)

	server := NewServer("localhost:8080", "", "", mgr, cfg)

	require.NotNil(t, server)
	assert.Empty(t, server.authUser)
	assert.Empty(t, server.authPass)
}

func TestSetupRoutes(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	mux := server.setupRoutes()

	require.NotNil(t, mux)

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

			mux.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestSetupRoutes_WithAuth(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	mux := server.setupRoutes()

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

			mux.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestHandleJobsRouter(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET returns jobs list",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST returns bad request",
			method:         http.MethodPost,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "PUT not allowed",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "DELETE not allowed",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/jobs", nil)
			rr := httptest.NewRecorder()

			server.handleJobsRouter(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestHandleJobsDetailRouter_BackupTrigger(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	req := httptest.NewRequest("POST", "/api/jobs/backup", nil)
	rr := httptest.NewRecorder()

	server.handleJobsDetailRouter(rr, req)

	// Should route to handleTriggerBackup
	// Expecting bad request because no body
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleJobsDetailRouter_RestoreTrigger(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	req := httptest.NewRequest("POST", "/api/jobs/restore", nil)
	rr := httptest.NewRecorder()

	server.handleJobsDetailRouter(rr, req)

	// Should route to handleTriggerRestore
	// Expecting bad request because no body
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleJobsDetailRouter_GetJob(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	req := httptest.NewRequest("GET", "/api/jobs/someid", nil)
	rr := httptest.NewRecorder()

	server.handleJobsDetailRouter(rr, req)

	// Should route to handleGetJob
	// Expecting 404 because job doesn't exist
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleJobsDetailRouter_CancelJob(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	req := httptest.NewRequest("DELETE", "/api/jobs/someid", nil)
	rr := httptest.NewRecorder()

	server.handleJobsDetailRouter(rr, req)

	// Should route to handleCancelJob
	// Expecting 404 because job doesn't exist
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleJobsDetailRouter_GetLogs(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	req := httptest.NewRequest("GET", "/api/jobs/someid/logs", nil)
	rr := httptest.NewRecorder()

	server.handleJobsDetailRouter(rr, req)

	// Should route to handleGetJobLogs
	// Expecting 404 because job doesn't exist
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleJobsDetailRouter_NotFound(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "empty job id",
			path: "/api/jobs/",
		},
		{
			name: "invalid path",
			path: "/api/jobs//something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()

			server.handleJobsDetailRouter(rr, req)

			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
}

func TestHandleJobsDetailRouter_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	// PUT not allowed on job detail
	req := httptest.NewRequest("PUT", "/api/jobs/someid", nil)
	rr := httptest.NewRecorder()

	server.handleJobsDetailRouter(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestShutdown_NoServer(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	// Should not error when httpServer is nil
	ctx := context.Background()
	err := server.Shutdown(ctx)

	assert.NoError(t, err)
}

func TestShutdown_WithServer(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:0", "admin", "secret", mgr, cfg) // Use port 0 for any available port

	// Create httpServer without actually listening
	mux := server.setupRoutes()
	server.httpServer = &http.Server{
		Addr:         server.addr,
		Handler:      mux,
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

	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	mux := server.setupRoutes()

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

			mux.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestServer_CORSHeaders(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	mux := server.setupRoutes()

	// Test CORS headers are set on API routes
	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "GET")
}

func TestServer_OptionsRequest(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, 2, 10)
	server := NewServer("localhost:8080", "admin", "secret", mgr, cfg)

	mux := server.setupRoutes()

	// Test OPTIONS (preflight) request
	req := httptest.NewRequest("OPTIONS", "/api/dashboard", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}
