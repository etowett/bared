package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/jobs"
	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

// streamTestServer spins up a running server whose log-stream route is wrapped
// in the real auth middleware, and returns it with a job to stream.
func streamTestServer(t *testing.T) (*Server, string) {
	t.Helper()

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
	server := NewServer(ServerOptions{
		Addr:       "localhost:8080",
		AuthUser:   "admin",
		AuthPass:   "secret",
		JobManager: mgr,
		Config:     cfg,
	})

	jobID, err := mgr.SubmitBackup(context.Background(), target, true)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	job, err := mgr.GetJob(jobID)
	require.NoError(t, err)
	job.Logs.Write("INFO", "streaming")

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(server.authMiddleware)
		r.Get("/api/jobs/{id}/logs/stream", server.handleStreamJobLogs)
	})

	testServer := httptest.NewServer(r)
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") +
		"/api/jobs/" + string(job.ID) + "/logs/stream"

	return server, wsURL
}

// dialWithSession opens the log stream authenticated by a session cookie — the
// browser's only option, since it cannot set an Authorization header on a
// WebSocket handshake.
func dialWithSession(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()

	header := http.Header{}
	header.Set("Cookie", sessionCookieName+"="+token)

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	t.Cleanup(func() { _ = ws.Close() })

	return ws
}

// expectClosed asserts the server hangs up within the deadline.
func expectClosed(t *testing.T, ws *websocket.Conn, why string) {
	t.Helper()

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			assert.False(t, isTimeout(err), "stream should have been closed: %s", why)
			return
		}
	}
}

func isTimeout(err error) bool {
	netErr, ok := errors.AsType[net.Error](err)
	return ok && netErr.Timeout()
}

func TestHandleStreamJobLogs_AuthenticatesWithSessionCookie(t *testing.T) {
	server, wsURL := streamTestServer(t)

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	ws := dialWithSession(t, wsURL, token)

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))
	var entry LogEntry
	require.NoError(t, ws.ReadJSON(&entry))
	assert.Contains(t, entry.Message, "streaming")
}

func TestHandleStreamJobLogs_RejectsUnauthenticatedHandshake(t *testing.T) {
	_, wsURL := streamTestServer(t)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// Logout must reach connections that are already streaming. The handshake is
// the only point where auth is checked, so without this a logged-out browser
// would keep receiving job logs until it disconnected on its own.
func TestHandleStreamJobLogs_LogoutClosesLiveStream(t *testing.T) {
	server, wsURL := streamTestServer(t)

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	ws := dialWithSession(t, wsURL, token)

	// Drain the backlog so we know the stream is live.
	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))
	var entry LogEntry
	require.NoError(t, ws.ReadJSON(&entry))

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	server.handleLogout(httptest.NewRecorder(), logoutReq)

	expectClosed(t, ws, "session was revoked by logout")
}

// The same must hold for the absolute TTL, otherwise "12 hour session" is only
// true of requests, not of the endpoint that streams for hours.
func TestHandleStreamJobLogs_ExpiryClosesLiveStream(t *testing.T) {
	server, wsURL := streamTestServer(t)

	now := time.Now()
	server.sessions.now = func() time.Time { return now }

	token, err := server.sessions.Issue("admin")
	require.NoError(t, err)

	ws := dialWithSession(t, wsURL, token)

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))
	var entry LogEntry
	require.NoError(t, ws.ReadJSON(&entry))

	// Cross the expiry and let the reaper run.
	now = now.Add(2 * defaultSessionTTL)
	server.sessions.sweep()

	expectClosed(t, ws, "session expired")
}

// Basic-auth streams have no session and must not be closed by anyone else's
// logout.
func TestHandleStreamJobLogs_BasicAuthStreamSurvivesLogout(t *testing.T) {
	server, wsURL := streamTestServer(t)

	header := http.Header{}
	header.Set("Authorization", "Basic YWRtaW46c2VjcmV0") // admin:secret

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	defer ws.Close()

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))
	var entry LogEntry
	require.NoError(t, ws.ReadJSON(&entry))

	// Someone else's session ends; this stream is unaffected.
	other, err := server.sessions.Issue("admin")
	require.NoError(t, err)
	server.sessions.Revoke(other)

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(300*time.Millisecond)))
	_, _, err = ws.ReadMessage()
	require.Error(t, err)
	assert.True(t, isTimeout(err), "a Basic-auth stream should stay open")
}
