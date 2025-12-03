package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/web"
)

// Server represents the HTTP API server
type Server struct {
	addr       string
	jobManager *jobs.Manager
	cfg        *config.Config
	httpServer *http.Server
	authUser   string
	authPass   string
}

// NewServer creates a new API server
func NewServer(addr, authUser, authPass string, jobManager *jobs.Manager, cfg *config.Config) *Server {
	return &Server{
		addr:       addr,
		authUser:   authUser,
		authPass:   authPass,
		jobManager: jobManager,
		cfg:        cfg,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting HTTP server on %s", s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		log.Println("Shutting down HTTP server...")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Health check (no auth required)
	mux.HandleFunc("/api/health", corsMiddleware(loggingMiddleware(s.handleHealth)))

	// API routes (require authentication)
	mux.HandleFunc("/api/dashboard", corsMiddleware(loggingMiddleware(s.basicAuthMiddleware(s.handleDashboard))))
	mux.HandleFunc("/api/targets", corsMiddleware(loggingMiddleware(s.basicAuthMiddleware(s.handleListTargets))))
	mux.HandleFunc("/api/jobs", corsMiddleware(loggingMiddleware(s.basicAuthMiddleware(s.handleJobsRouter))))
	mux.HandleFunc("/api/jobs/", corsMiddleware(loggingMiddleware(s.basicAuthMiddleware(s.handleJobsDetailRouter))))

	// Serve React SPA for all non-API routes
	mux.Handle("/", web.GetHandler())

	return mux
}

// handleJobsRouter routes /api/jobs requests
func (s *Server) handleJobsRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListJobs(w, r)
	case http.MethodPost:
		// Check sub-path for backup or restore
		// This is handled by specific routes in production
		respondError(w, http.StatusBadRequest, "Use /api/jobs/backup or /api/jobs/restore")
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleJobsDetailRouter routes /api/jobs/{id}/* requests
func (s *Server) handleJobsDetailRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle backup/restore triggers
	if strings.HasSuffix(path, "/backup") {
		s.handleTriggerBackup(w, r)
		return
	}
	if strings.HasSuffix(path, "/restore") {
		s.handleTriggerRestore(w, r)
		return
	}

	// Handle job-specific routes
	if strings.Contains(path, "/logs/stream") {
		s.handleStreamJobLogs(w, r)
		return
	}
	if strings.HasSuffix(path, "/logs") {
		s.handleGetJobLogs(w, r)
		return
	}

	// Get single job
	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[3] != "" {
		switch r.Method {
		case http.MethodGet:
			s.handleGetJob(w, r)
		case http.MethodDelete:
			s.handleCancelJob(w, r)
		default:
			respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	respondError(w, http.StatusNotFound, "Not found")
}
