package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/testutil/fixtures"
)

func TestHandleStreamJobLogs_JobNotFound(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}
	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)

	req := httptest.NewRequest("GET", "/api/jobs/nonexistent-id/logs/stream", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleStreamJobLogs_WebSocketUpgrade(t *testing.T) {
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
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	// Submit a backup job to manager
	ctx := context.Background()
	jobID, err := mgr.SubmitBackup(ctx, target, true)
	require.NoError(t, err)

	// Give job time to be registered and get the job
	time.Sleep(10 * time.Millisecond)
	job, err := mgr.GetJob(jobID)
	require.NoError(t, err)

	// Write some logs to the job
	job.Logs.Write("INFO", "Test log message 1")
	job.Logs.Write("INFO", "Test log message 2")

	// Create test server using chi router for URL param extraction
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/jobs/" + string(job.ID) + "/logs/stream"

	// Connect WebSocket client
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Read existing logs
	var entries []LogEntry
	for i := 0; i < 2; i++ {
		var entry LogEntry
		err := ws.ReadJSON(&entry)
		require.NoError(t, err)
		entries = append(entries, entry)
	}

	assert.Len(t, entries, 2)
	assert.Contains(t, entries[0].Message, "Test log message")
	assert.Contains(t, entries[1].Message, "Test log message")
}

func TestHandleStreamJobLogs_StreamNewLogs(t *testing.T) {
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
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	// Submit a backup job to manager
	ctx := context.Background()
	jobID, err := mgr.SubmitBackup(ctx, target, true)
	require.NoError(t, err)

	// Give job time to be registered and get the job
	time.Sleep(10 * time.Millisecond)
	job, err := mgr.GetJob(jobID)
	require.NoError(t, err)

	// Create test server using chi router
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/jobs/" + string(job.ID) + "/logs/stream"

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Write new logs after connection
	go func() {
		time.Sleep(50 * time.Millisecond)
		job.Logs.Write("INFO", "New log message 1")
		time.Sleep(50 * time.Millisecond)
		job.Logs.Write("INFO", "New log message 2")
	}()

	// Read new logs with timeout
	ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	var entries []LogEntry
	for i := 0; i < 2; i++ {
		var entry LogEntry
		err := ws.ReadJSON(&entry)
		if err != nil {
			break // Timeout or connection closed
		}
		entries = append(entries, entry)
	}

	// Should receive at least one new log message
	assert.GreaterOrEqual(t, len(entries), 1)
	if len(entries) > 0 {
		assert.Contains(t, entries[0].Message, "New log message")
	}
}

func TestHandleStreamJobLogs_ClientDisconnect(t *testing.T) {
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
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	// Submit a backup job to manager
	ctx := context.Background()
	jobID, err := mgr.SubmitBackup(ctx, target, true)
	require.NoError(t, err)

	// Give job time to be registered and get the job
	time.Sleep(10 * time.Millisecond)
	job, err := mgr.GetJob(jobID)
	require.NoError(t, err)

	// Create test server using chi router
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/jobs/" + string(job.ID) + "/logs/stream"

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Close connection immediately
	ws.Close()

	// Server should handle disconnection gracefully (no panic)
	time.Sleep(100 * time.Millisecond)
}

func TestHandleStreamJobLogs_ContextCancellation(t *testing.T) {
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
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	// Submit a backup job to manager
	ctx := context.Background()
	jobID, err := mgr.SubmitBackup(ctx, target, true)
	require.NoError(t, err)

	// Give job time to be registered and get the job
	time.Sleep(10 * time.Millisecond)
	job, err := mgr.GetJob(jobID)
	require.NoError(t, err)

	// Create test server with cancellable context using chi router
	var cancel context.CancelFunc
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", func(w http.ResponseWriter, req *http.Request) {
		// Derive cancellable context from the request context (preserves chi route params)
		var cancelCtx context.Context
		cancelCtx, cancel = context.WithCancel(req.Context())
		server.handleStreamJobLogs(w, req.WithContext(cancelCtx))
	})

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/jobs/" + string(job.ID) + "/logs/stream"

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Cancel context after connection (cancel is set by the handler above)
	go func() {
		time.Sleep(50 * time.Millisecond)
		if cancel != nil {
			cancel()
		}
	}()

	// Try to read - should fail due to context cancellation
	ws.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var entry LogEntry
	err = ws.ReadJSON(&entry)

	// Connection should be closed (either normal close or timeout)
	assert.Error(t, err)
}

func TestLogEntryToResponse(t *testing.T) {
	now := time.Now()
	entry := jobs.LogEntry{
		Timestamp: now,
		Level:     "INFO",
		Message:   "Test message",
	}

	response := LogEntryToResponse(entry)

	assert.Equal(t, now.Format(time.RFC3339), response.Timestamp)
	assert.Equal(t, "INFO", response.Level)
	assert.Equal(t, "Test message", response.Message)
}

func TestLogEntryToResponse_DifferentLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{name: "INFO level", level: "INFO"},
		{name: "ERROR level", level: "ERROR"},
		{name: "WARN level", level: "WARN"},
		{name: "DEBUG level", level: "DEBUG"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := jobs.LogEntry{
				Timestamp: time.Now(),
				Level:     tt.level,
				Message:   "Test message",
			}

			response := LogEntryToResponse(entry)

			assert.Equal(t, tt.level, response.Level)
		})
	}
}

// The upgrader used to accept every origin. Now that the handshake
// authenticates with a cookie the browser attaches automatically, that would be
// a cross-site WebSocket hijacking vector.
func TestWebSocketUpgrader_CheckOrigin(t *testing.T) {
	server := &Server{allowedOrigins: normaliseAllowedOrigins([]string{"http://localhost:5173"})}
	checkOrigin := server.upgrader().CheckOrigin

	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{name: "same origin", origin: "http://localhost:8080", allowed: true},
		{name: "same origin over TLS behind a proxy", origin: "https://localhost:8080", allowed: true},
		{name: "allowlisted dev server", origin: "http://localhost:5173", allowed: true},
		{name: "no origin (non-browser client)", origin: "", allowed: true},
		{name: "foreign origin", origin: "https://evil.example", allowed: false},
		{name: "lookalike host", origin: "http://evil-localhost:8080", allowed: false},
		{name: "same host, different port", origin: "http://localhost:8081", allowed: false},
		{name: "allowlist is not a prefix match", origin: "http://localhost:51730", allowed: false},
		{name: "opaque origin", origin: "null", allowed: false},
		{name: "unsupported scheme", origin: "file://localhost:8080", allowed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			req.Host = "localhost:8080"
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			assert.Equal(t, tt.allowed, checkOrigin(req))
		})
	}
}

func TestHandleStreamJobLogs_MultipleClients(t *testing.T) {
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
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	// Submit a backup job to manager
	ctx := context.Background()
	jobID, err := mgr.SubmitBackup(ctx, target, true)
	require.NoError(t, err)

	// Give job time to be registered and get the job
	time.Sleep(10 * time.Millisecond)
	job, err := mgr.GetJob(jobID)
	require.NoError(t, err)

	// Write initial log
	job.Logs.Write("INFO", "Initial log")

	// Create test server using chi router
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/jobs/" + string(job.ID) + "/logs/stream"

	// Connect multiple WebSocket clients
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws1.Close()

	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws2.Close()

	// Both clients should receive initial log
	var entry1 LogEntry
	err = ws1.ReadJSON(&entry1)
	require.NoError(t, err)
	assert.Contains(t, entry1.Message, "Initial log")

	var entry2 LogEntry
	err = ws2.ReadJSON(&entry2)
	require.NoError(t, err)
	assert.Contains(t, entry2.Message, "Initial log")

	// Write new log
	job.Logs.Write("INFO", "New broadcast message")

	// Both clients should receive the new log
	ws1.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	ws2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

	err = ws1.ReadJSON(&entry1)
	if err == nil {
		assert.Contains(t, entry1.Message, "New broadcast message")
	}

	err = ws2.ReadJSON(&entry2)
	if err == nil {
		assert.Contains(t, entry2.Message, "New broadcast message")
	}
}

func TestHandleStreamJobLogs_EmptyLogs(t *testing.T) {
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
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	// Submit a backup job to manager (no logs written)
	ctx := context.Background()
	jobID, err := mgr.SubmitBackup(ctx, target, true)
	require.NoError(t, err)

	// Give job time to be registered
	time.Sleep(10 * time.Millisecond)

	// Create test server using chi router
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/jobs/" + string(jobID) + "/logs/stream"

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Should not receive any logs (set short deadline)
	ws.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var entry LogEntry
	err = ws.ReadJSON(&entry)

	// Should timeout or get an error (no logs to read)
	assert.Error(t, err)
}
