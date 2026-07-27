package api

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
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

	// Dashboard session auth
	sessions       *sessionStore
	sessionTTL     time.Duration
	allowedOrigins []string
	secureCookies  bool

	// Brute-force protection for the one unauthenticated endpoint that checks
	// a password.
	loginLimiter   *ipRateLimiter
	trustedProxies []netip.Prefix

	// failedLoginDelay is a field rather than a constant only so tests can
	// zero it; nothing configures it at runtime.
	failedLoginDelay time.Duration
}

// ServerOptions configures a Server. It replaced a positional constructor that
// had grown to eight parameters.
type ServerOptions struct {
	Addr          string
	AuthUser      string
	AuthPass      string
	JobManager    *jobs.Manager
	Config        *config.Config
	ConfigService *configservice.Service
	ConfigLoader  *configservice.Loader
	ReloadChan    chan struct{}

	// SessionTTL is the absolute lifetime of a dashboard session. Zero means
	// defaultSessionTTL.
	SessionTTL time.Duration

	// AllowedOrigins are extra origins permitted for CORS, CSRF, and the
	// WebSocket handshake. Same-origin is always allowed; this exists for the
	// Vite dev server and for reverse-proxy deployments.
	AllowedOrigins []string

	// SecureCookies forces the Secure attribute on the session cookie for
	// operators terminating TLS in front of the daemon. It is opt-in because
	// X-Forwarded-Proto is client-controlled and cannot be trusted, and because
	// a Secure cookie on a plain-HTTP install would silently break login.
	SecureCookies bool

	// TrustedProxies are the addresses or CIDRs whose X-Forwarded-For header
	// may be believed when attributing a login attempt to an IP. Empty — the
	// default — means the header is ignored entirely and only the immediate
	// peer counts. It is opt-in for the same reason SecureCookies is: the
	// header is client-controlled, and believing it would let an attacker mint
	// a fresh identity per request and stroll past the per-IP limiter.
	TrustedProxies []string
}

// NewServer creates a new API server
func NewServer(opts ServerOptions) *Server {
	ttl := opts.SessionTTL
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}

	return &Server{
		addr:           opts.Addr,
		authUser:       opts.AuthUser,
		authPass:       opts.AuthPass,
		jobManager:     opts.JobManager,
		cfg:            opts.Config,
		configService:  opts.ConfigService,
		configLoader:   opts.ConfigLoader,
		reloadChan:     opts.ReloadChan,
		sessions:       newSessionStore(ttl),
		sessionTTL:     ttl,
		allowedOrigins: normaliseAllowedOrigins(opts.AllowedOrigins),
		secureCookies:  opts.SecureCookies,
		loginLimiter:   newLoginRateLimiter(),
		trustedProxies: parseTrustedProxies(opts.TrustedProxies),

		failedLoginDelay: failedLoginDelay,
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

	// Reap expired sessions so an absolute TTL also reaches live WebSockets,
	// which are only authenticated at the handshake.
	s.sessions.startSweeper(sessionSweepInterval)

	// Reap idle login buckets so the per-IP map stays bounded.
	s.loginLimiter.startSweeper(loginLimiterSweepInterval)

	logger.InfoS("Starting HTTP server",
		"component", "api",
		"address", s.addr)
	if !s.secureCookies {
		logger.WarnS("Session cookies are not marked Secure; serve the dashboard over TLS "+
			"or pass --http-secure-cookies when terminating TLS in front of the daemon",
			"component", "api")
	}
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.sessions.stopSweeper()
	s.loginLimiter.stopSweeper()

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
	r.Use(s.corsMiddleware)

	// Health check (no auth required)
	r.Get("/api/health", s.handleHealth)

	// Session endpoints. Login and logout are unauthenticated by definition —
	// login establishes the session, and logout must stay usable with an
	// already-expired one so the browser can clear its cookie.
	r.Post("/api/login", s.handleLogin)
	r.Post("/api/logout", s.handleLogout)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		// Order matters: csrfMiddleware reads the identity authMiddleware attaches.
		r.Use(s.csrfMiddleware)

		r.Get("/api/me", s.handleMe)

		r.Get("/api/dashboard", s.handleDashboard)
		r.Get("/api/targets", s.handleListTargets)
		r.Get("/api/restore-targets", s.handleListRestoreTargets)

		// Jobs
		r.Get("/api/jobs", s.handleListJobs)
		r.Post("/api/jobs/backup", s.handleTriggerBackup)
		r.Post("/api/jobs/restore", s.handleTriggerRestore)
		r.Route("/api/jobs/{id}", func(r chi.Router) {
			r.Get("/", s.handleGetJob)
			r.Delete("/", s.handleCancelJob)
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
				r.Post("/import", s.handleImportConfig)
				r.Get("/source", s.handleGetConfigSource)
			})
		}
	})

	// Serve React SPA for all non-API routes
	r.NotFound(web.GetHandler().ServeHTTP)

	return r
}
