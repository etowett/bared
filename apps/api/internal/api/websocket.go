package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/etowett/bared/apps/api/internal/jobs"
	"github.com/etowett/bared/apps/api/internal/util"
)

// upgrader builds the WebSocket upgrader for this server.
//
// CheckOrigin rejects foreign origins. Now that the handshake authenticates via
// a cookie the browser attaches automatically, an unrestricted upgrader would
// let any page on the internet open an authenticated log stream against a
// reachable instance (cross-site WebSocket hijacking). A missing Origin is
// accepted — non-browser clients don't send one, and they cannot be driven by a
// hostile page.
func (s *Server) upgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			return s.originAllowed(origin, r.Host)
		},
	}
}

// handleStreamJobLogs handles WebSocket connections for log streaming
func (s *Server) handleStreamJobLogs(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()

	jobID := jobs.JobID(chi.URLParam(r, "id"))

	// Get job
	job, err := s.jobManager.GetJob(jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	// The session backing this request, if it authenticated with a cookie. The
	// handshake is the only point where auth is checked, so the stream watches
	// the session to learn about logout and expiry.
	var sessionDone <-chan struct{}
	if auth, ok := authFromContext(r.Context()); ok {
		sessionDone = auth.sess.Done()
	}

	// Upgrade connection to WebSocket
	upgrader := s.upgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.ErrorS("Failed to upgrade WebSocket connection",
			"component", "api",
			"job_id", jobID,
			"error", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.WarnS("Failed to close WebSocket connection",
				"component", "api",
				"job_id", jobID,
				"error", err)
		}
	}()

	logger.InfoS("WebSocket client connected",
		"component", "api",
		"job_id", jobID)

	// Subscribe to log buffer
	logCh := job.Logs.Subscribe()
	defer job.Logs.Unsubscribe(logCh)

	// Send existing logs first
	existingLogs := job.Logs.GetAll()
	for _, entry := range existingLogs {
		apiEntry := LogEntryToResponse(entry)
		if err := conn.WriteJSON(apiEntry); err != nil {
			logger.ErrorS("Failed to send existing log entry",
				"component", "api",
				"job_id", jobID,
				"error", err)
			return
		}
	}

	// Setup ping ticker for keepalive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Create done channel for cleanup
	done := make(chan struct{})

	// Start goroutine to read from client (to detect disconnection)
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Stream new logs
	for {
		select {
		case entry := <-logCh:
			// Send new log entry
			apiEntry := LogEntryToResponse(entry)
			if err := conn.WriteJSON(apiEntry); err != nil {
				logger.ErrorS("Failed to send log entry",
					"component", "api",
					"job_id", jobID,
					"error", err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.WarnS("Failed to send ping",
					"component", "api",
					"job_id", jobID,
					"error", err)
				return
			}

		case <-done:
			// Client disconnected
			logger.InfoS("WebSocket client disconnected",
				"component", "api",
				"job_id", jobID)
			return

		case <-sessionDone:
			// Session revoked (logout) or expired — stop streaming rather than
			// serving job logs to a browser that is no longer authenticated.
			logger.InfoS("WebSocket closed: session ended",
				"component", "api",
				"job_id", jobID)
			if err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session ended")); err != nil {
				logger.WarnS("Failed to send close frame",
					"component", "api",
					"job_id", jobID,
					"error", err)
			}
			return

		case <-r.Context().Done():
			// Request context cancelled
			return
		}
	}
}
