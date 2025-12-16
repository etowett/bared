package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"bared/internal/jobs"
	"bared/internal/util"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		// Allow all origins for now (can be restricted in production)
		return true
	},
}

// handleStreamJobLogs handles WebSocket connections for log streaming
func (s *Server) handleStreamJobLogs(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()

	// Extract job ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}
	jobIDStr := parts[3] // /api/jobs/{id}/logs/stream

	jobID := jobs.JobID(jobIDStr)

	// Get job
	job, err := s.jobManager.GetJob(jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Upgrade connection to WebSocket
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

		case <-r.Context().Done():
			// Request context cancelled
			return
		}
	}
}
