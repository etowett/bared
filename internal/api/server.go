package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"bared/internal/config"
	"bared/internal/configservice"
	"bared/internal/jobs"
	"bared/internal/util"
	"bared/internal/web"
)

// Server represents the HTTP API server
type Server struct {
	addr          string
	jobManager    *jobs.Manager
	cfg           *config.Config
	httpServer    *http.Server
	authUser      string
	authPass      string
	configService *configservice.Service
	configLoader  *configservice.Loader
	reloadChan    chan struct{}
}

// NewServer creates a new API server
func NewServer(addr, authUser, authPass string, jobManager *jobs.Manager, cfg *config.Config,
	configService *configservice.Service, configLoader *configservice.Loader, reloadChan chan struct{}) *Server {
	return &Server{
		addr:          addr,
		authUser:      authUser,
		authPass:      authPass,
		jobManager:    jobManager,
		cfg:           cfg,
		configService: configService,
		configLoader:  configLoader,
		reloadChan:    reloadChan,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	logger := util.GetLogger()
	r := s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.InfoS("Starting HTTP server",
		"component", "api",
		"address", s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		logger := util.GetLogger()
		logger.InfoS("Shutting down HTTP server",
			"component", "api")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(corsMiddleware)

	// Health check (no auth required)
	r.Get("/api/health", s.handleHealth)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(s.basicAuthMiddleware)

		r.Get("/api/dashboard", s.handleDashboard)
		r.Get("/api/targets", s.handleListTargets)
		r.Get("/api/restore-targets", s.handleListRestoreTargets)

		// Jobs
		r.Get("/api/jobs", s.handleListJobs)
		r.Route("/api/jobs/{id}", func(r chi.Router) {
			r.Get("/", s.handleGetJob)
			r.Delete("/", s.handleCancelJob)
			r.Post("/backup", s.handleTriggerBackup)
			r.Post("/restore", s.handleTriggerRestore)
			r.Get("/logs", s.handleGetJobLogs)
			r.Get("/logs/stream", s.handleStreamJobLogs)
		})

		// Config management routes
		if s.configService != nil {
			r.Route("/api/config", func(r chi.Router) {
				// Storages
				r.Get("/storages", s.handleListStorages)
				r.Post("/storages", s.handleCreateStorage)
				r.Get("/storages/{name}", s.handleGetStorage)
				r.Put("/storages/{name}", s.handleUpdateStorage)
				r.Delete("/storages/{name}", s.handleDeleteStorage)

				// Notifiers
				r.Get("/notifiers", s.handleListNotifiers)
				r.Post("/notifiers", s.handleCreateNotifier)
				r.Put("/notifiers/{name}", s.handleUpdateNotifier)
				r.Delete("/notifiers/{name}", s.handleDeleteNotifier)

				// Targets
				r.Get("/targets", s.handleListTargetsConfig)
				r.Post("/targets", s.handleCreateTarget)
				r.Put("/targets/{name}", s.handleUpdateTarget)
				r.Patch("/targets/{name}/schedule", s.handleUpdateTargetSchedule)
				r.Delete("/targets/{name}", s.handleDeleteTarget)

				// Restore targets
				r.Get("/restore-targets", s.handleListRestoreTargetsConfig)
				r.Post("/restore-targets", s.handleCreateRestoreTarget)
				r.Put("/restore-targets/{name}", s.handleUpdateRestoreTarget)
				r.Delete("/restore-targets/{name}", s.handleDeleteRestoreTarget)

				// Global config
				r.Get("/global", s.handleGetGlobalConfig)
				r.Put("/global/{key}", s.handleUpdateGlobalConfig)

				// Utility endpoints
				r.Post("/migrate", s.handleMigrateConfig)
				r.Post("/reload", s.handleReloadConfig)
				r.Get("/source", s.handleGetConfigSource)
			})
		}
	})

	// Serve React SPA for all non-API routes
	r.NotFound(web.GetHandler().ServeHTTP)

	return r
}
