// Package daemon implements the background process for scheduling and running jobs.
package daemon

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"bared/internal/api"
	"bared/internal/config"
	"bared/internal/configservice"
	"bared/internal/encryption"
	"bared/internal/jobs"
	"bared/internal/notify"
	"bared/internal/persistence"
	"bared/internal/util"
)

// Daemon represents the backup daemon
type Daemon struct {
	cfg           *config.Config
	scheduler     *cron.Cron
	jobManager    *jobs.Manager
	store         jobs.JobStore
	ctx           context.Context
	cancel        context.CancelFunc
	configService *configservice.Service
	configLoader  *configservice.Loader
	reloadChan    chan struct{}

	// HTTP server configuration (optional)
	httpAddr  string
	authUser  string
	authPass  string
	apiServer *api.Server

	// Job configuration
	maxConcurrentJobs int
	jobHistorySize    int
	shutdownTimeout   time.Duration
}

// Option is a functional option for configuring the daemon
type Option func(*Daemon)

// WithHTTP enables the HTTP server
func WithHTTP(addr, user, pass string) Option {
	return func(d *Daemon) {
		d.httpAddr = addr
		d.authUser = user
		d.authPass = pass
	}
}

// WithMaxConcurrentJobs sets the maximum number of concurrent jobs
func WithMaxConcurrentJobs(n int) Option {
	return func(d *Daemon) {
		d.maxConcurrentJobs = n
	}
}

// WithJobHistorySize sets the job history size per target
func WithJobHistorySize(n int) Option {
	return func(d *Daemon) {
		d.jobHistorySize = n
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(d *Daemon) {
		d.shutdownTimeout = timeout
	}
}

// New creates a new daemon instance
func New(cfg *config.Config, opts ...Option) *Daemon {
	logger := util.GetLogger()
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize persistence
	var store jobs.JobStore
	var db *sql.DB
	if cfg.Persistence != nil && cfg.Persistence.Enabled {
		// Default to sqlite if not specified but enabled
		driver := cfg.Persistence.Type
		if driver == "" {
			driver = "sqlite3"
		}

		dsn := cfg.Persistence.DSN
		if dsn == "" && driver == "sqlite3" {
			dsn = "bared.db"
		}

		sqlStore, err := persistence.NewSQLStore(driver, dsn)
		sanitized := persistence.SanitizeDSN(driver, dsn)
		if err != nil {
			logger.WarnS("Failed to initialize persistence. Running in-memory only.",
				"component", "daemon",
				"driver", driver,
				"dsn", sanitized,
				"error", err)
			store = nil // Explicitly set to nil to avoid nil pointer in interface
		} else {
			logger.InfoS("Persistence enabled",
				"component", "daemon",
				"driver", driver,
				"dsn", sanitized)
			store = sqlStore // Only assign if successful
			db = sqlStore.DB()
		}
	}

	// Initialize encryption service and config service
	var configSvc *configservice.Service
	var configLoader *configservice.Loader
	var reloadChan chan struct{}

	if db != nil {
		// Initialize encryption key (check env var, fallback to DB)
		encryptionKey, err := initializeEncryptionKey(db, logger)
		if err != nil {
			logger.WarnS("Failed to initialize encryption key. Config management disabled.",
				"component", "daemon",
				"error", err)
		} else {
			encryptionSvc, err := encryption.NewService(encryptionKey)
			if err != nil {
				logger.WarnS("Failed to create encryption service. Config management disabled.",
					"component", "daemon",
					"error", err)
			} else {
				configSvc = configservice.NewService(db, encryptionSvc)
				configLoader = configservice.NewLoader(configSvc, cfg)
				reloadChan = make(chan struct{}, 1)
				logger.InfoS("Config management service initialized", "component", "daemon")
			}
		}
	}

	// Create daemon with defaults
	d := &Daemon{
		cfg:               cfg,
		scheduler:         cron.New(),
		ctx:               ctx,
		cancel:            cancel,
		store:             store,
		configService:     configSvc,
		configLoader:      configLoader,
		reloadChan:        reloadChan,
		maxConcurrentJobs: 3,
		jobHistorySize:    10,
		shutdownTimeout:   1 * time.Hour,
	}

	// Apply options
	for _, opt := range opts {
		opt(d)
	}

	// Create job manager
	d.jobManager = jobs.NewManager(cfg, store, d.maxConcurrentJobs, d.jobHistorySize)

	// Initialize HTTP server if configured
	if d.httpAddr != "" {
		logger.InfoS("Creating HTTP server",
			"component", "daemon",
			"address", d.httpAddr)
		d.apiServer = api.NewServer(d.httpAddr, d.authUser, d.authPass, d.jobManager, cfg,
			configSvc, configLoader, reloadChan)
	}

	return d
}

// initializeEncryptionKey initializes the encryption key from env var or DB
func initializeEncryptionKey(db *sql.DB, logger *util.Logger) ([]byte, error) {
	// Check for environment variable first
	if envKey := os.Getenv("BARED_ENCRYPTION_KEY"); envKey != "" {
		logger.InfoS("Using encryption key from environment variable", "component", "daemon")
		key, err := base64.StdEncoding.DecodeString(envKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode BARED_ENCRYPTION_KEY: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("BARED_ENCRYPTION_KEY must be 32 bytes (64 hex chars), got %d", len(key))
		}
		return key, nil
	}

	// Check DB for existing active key
	var keyData string
	err := db.QueryRow(`SELECT key_data FROM encryption_keys WHERE active = true ORDER BY created_at DESC LIMIT 1`).Scan(&keyData)
	if err == nil {
		logger.InfoS("Using encryption key from database", "component", "daemon")
		return base64.StdEncoding.DecodeString(keyData)
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query encryption keys: %w", err)
	}

	// Generate new key and store in DB
	logger.InfoS("Generating new encryption key", "component", "daemon")
	key, err := encryption.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	keyData = base64.StdEncoding.EncodeToString(key)
	_, err = db.Exec(`INSERT INTO encryption_keys (key_data, active, created_at) VALUES (?, true, ?)`,
		keyData, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to store encryption key: %w", err)
	}

	logger.InfoS("Generated and stored new encryption key", "component", "daemon")
	return key, nil
}

// Start starts the daemon and scheduler
func (d *Daemon) Start() error {
	logger := util.GetLogger()
	logger.InfoS("Starting BareD daemon", "component", "daemon")

	// Clean up orphaned temp files from previous runs
	logger.InfoS("Cleaning up orphaned temp files", "component", "daemon")
	if err := util.CleanupOrphanedTempFiles(); err != nil {
		logger.WarnS("Failed to cleanup orphaned temp files",
			"component", "daemon",
			"error", err)
		// Don't fail startup if cleanup fails
	}

	// Recover orphaned jobs from previous runs (e.g., after crash/OOM)
	logger.InfoS("Recovering orphaned jobs", "component", "daemon")
	if err := d.RecoverOrphanedJobs(); err != nil {
		logger.WarnS("Failed to recover orphaned jobs",
			"component", "daemon",
			"error", err)
		// Don't fail startup if recovery fails
	}

	// Start job manager worker pool
	d.jobManager.Start()

	// Start cleanup routine for old jobs (every 3 hours, remove jobs older than 72h)
	d.jobManager.StartCleanupRoutine(3*time.Hour, 72*time.Hour)

	// Start HTTP server if configured
	if d.apiServer != nil {
		go func() {
			if err := d.apiServer.Start(); err != nil {
				logger.ErrorS("HTTP server error",
					"component", "daemon",
					"error", err)
			}
		}()
		logger.InfoS("HTTP server started",
			"component", "daemon",
			"address", d.httpAddr)
	}

	// Load runtime configuration (DB first, then YAML fallback)
	runtimeCfg, source, err := d.loadRuntimeConfig()
	if err != nil {
		return fmt.Errorf("failed to load runtime config: %w", err)
	}
	logger.InfoS("Loaded runtime configuration",
		"component", "daemon",
		"source", source)

	// Schedule all targets that have a schedule configured
	scheduledCount := d.scheduleAllTargets(runtimeCfg)

	// Require at least one schedule OR HTTP server to be configured
	if scheduledCount == 0 && d.httpAddr == "" {
		return fmt.Errorf("no targets with schedules configured and no HTTP server enabled. " +
			"Either configure schedules for cron-based backups or enable HTTP server " +
			"(--http flag) for API/web-based operations")
	}

	// Start the scheduler only if there are scheduled targets
	if scheduledCount > 0 {
		d.scheduler.Start()
		logger.InfoS("Scheduler started",
			"component", "daemon",
			"scheduled_targets", scheduledCount)
	} else {
		logger.InfoS("Scheduler disabled - no targets with schedules configured",
			"component", "daemon")
	}

	// Start hot reload listener if config service is available
	if d.reloadChan != nil {
		go d.hotReloadListener()
		logger.InfoS("Hot reload listener started", "component", "daemon")
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for signals
	for {
		sig := <-sigChan

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			logger.InfoS("Received shutdown signal",
				"component", "daemon",
				"signal", sig.String())
			return d.Stop()

		case syscall.SIGHUP:
			logger.InfoS("Received SIGHUP signal, reloading configuration",
				"component", "daemon")
			if err := d.Reload(); err != nil {
				logger.ErrorS("Failed to reload configuration",
					"component", "daemon",
					"error", err)
			}
		}
	}
}

// RecoverOrphanedJobs finds and marks jobs left in "running" or "queued" state
// from previous daemon runs (e.g., after crash or OOM kill) as failed
func (d *Daemon) RecoverOrphanedJobs() error {
	logger := util.GetLogger()
	ctx := context.Background()

	// Skip recovery if persistence is disabled
	if d.store == nil {
		logger.DebugS("Skipping orphaned job recovery - persistence disabled",
			"component", "daemon")
		return nil
	}

	// Find jobs in "running" state from previous run
	runningJobs, err := d.store.ListJobs(ctx, jobs.JobFilter{
		Status: jobs.JobStatusRunning,
		Limit:  1000, // Get up to 1000 orphaned jobs
	})
	if err != nil {
		return fmt.Errorf("failed to query running jobs: %w", err)
	}

	// Find jobs in "queued" state from previous run
	queuedJobs, err := d.store.ListJobs(ctx, jobs.JobFilter{
		Status: jobs.JobStatusQueued,
		Limit:  1000,
	})
	if err != nil {
		return fmt.Errorf("failed to query queued jobs: %w", err)
	}

	orphaned := append(runningJobs, queuedJobs...)

	if len(orphaned) > 0 {
		logger.InfoS("Found orphaned jobs from previous run",
			"component", "daemon",
			"count", len(orphaned))
	}

	// Mark each as failed with crash indicator
	for _, job := range orphaned {
		job.Status = jobs.JobStatusFailed
		job.Error = "Job interrupted by daemon shutdown or crash"
		now := time.Now()
		job.CompletedAt = &now

		if err := d.store.UpdateJob(ctx, job); err != nil {
			logger.ErrorS("Failed to mark orphaned job as failed",
				"component", "daemon",
				"job_id", job.ID,
				"error", err)
		} else {
			logger.InfoS("Marked orphaned job as failed",
				"component", "daemon",
				"job_id", job.ID,
				"target", job.TargetName,
				"type", job.Type)
		}
	}

	return nil
}

// Stop stops the daemon gracefully
func (d *Daemon) Stop() error {
	logger := util.GetLogger()
	logger.InfoS("Stopping daemon gracefully", "component", "daemon")

	// 1. Stop scheduler (wait for running cron jobs to complete)
	logger.InfoS("Stopping scheduler", "component", "daemon")
	ctx := d.scheduler.Stop()
	<-ctx.Done()
	logger.InfoS("Scheduler stopped", "component", "daemon")

	// 2. Shutdown HTTP server
	if d.apiServer != nil {
		logger.InfoS("Stopping HTTP server", "component", "daemon")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.apiServer.Shutdown(shutdownCtx); err != nil {
			logger.ErrorS("HTTP server shutdown error",
				"component", "daemon",
				"error", err)
		}
		logger.InfoS("HTTP server stopped", "component", "daemon")
	}

	// 3. Shutdown job manager (wait for running jobs with timeout)
	logger.InfoS("Waiting for running jobs to complete",
		"component", "daemon",
		"timeout", d.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), d.shutdownTimeout)
	defer cancel()

	if err := d.jobManager.Shutdown(shutdownCtx); err != nil {
		logger.WarnS("Job manager shutdown timed out",
			"component", "daemon",
			"error", err)
		logger.WarnS("Some jobs may have been terminated",
			"component", "daemon")
	} else {
		logger.InfoS("All jobs completed successfully",
			"component", "daemon")
	}

	// 4. Close persistence if active
	if d.store != nil {
		if err := d.store.Close(); err != nil {
			logger.ErrorS("Error closing persistence layer",
				"component", "daemon",
				"error", err)
		}
	}

	// 5. Cancel daemon context
	d.cancel()

	logger.InfoS("Daemon stopped", "component", "daemon")
	return nil
}

// Reload reloads the configuration and reschedules targets
func (d *Daemon) Reload() error {
	logger := util.GetLogger()
	// TODO: Implement configuration reload
	// This would involve:
	// 1. Loading new configuration
	// 2. Stopping scheduler
	// 3. Rescheduling targets with new config
	// 4. Starting scheduler
	logger.InfoS("Configuration reload not yet fully implemented",
		"component", "daemon")
	return nil
}

// scheduleTarget adds a target to the scheduler
func (d *Daemon) scheduleTarget(target *config.Target) error {
	// Create a closure that captures the target
	targetCopy := target
	job := func() {
		// Use distributed locking if persistence is enabled
		if d.store != nil {
			// Lock key format: cron:target_name:YYYYMMDDHHMM
			// Bucketed to minute to allow one run per minute per target
			now := time.Now()
			lockKey := fmt.Sprintf("cron:%s:%s", targetCopy.Name, now.Format("200601021504"))
			ttl := 1 * time.Hour // Hold lock for an hour (simulates "job ran")

			logger := util.GetLogger()
			acquired, err := d.store.AcquireLock(d.ctx, lockKey, ttl)
			if err != nil {
				logger.ErrorS("Failed to acquire lock",
					"component", "daemon",
					"target", targetCopy.Name,
					"error", err)
				return // Fail safe
			}
			if !acquired {
				logger.InfoS("Skipping scheduled run (lock held by another instance)",
					"component", "daemon",
					"target", targetCopy.Name)
				return
			}

			// Note: We don't release the lock immediately. We want it to persist for the
			// duration of the "bucket" (the minute) so no other pod picks it up.
			// Actually, if we want to run once per schedule tick, holding it is correct.
			// The TTL handles the eventual cleanup.
		}

		logger := util.GetLogger()
		logger.InfoS("Starting scheduled backup",
			"component", "daemon",
			"target", targetCopy.Name)

		// Submit job to job manager
		jobID, err := d.jobManager.SubmitBackup(d.ctx, targetCopy, false)
		if err != nil {
			logger.ErrorS("Failed to submit scheduled backup",
				"component", "daemon",
				"target", targetCopy.Name,
				"error", err)

			// Send failure notification
			d.sendScheduleFailureNotification(d.ctx, targetCopy.Name, err)
			return
		}

		logger.InfoS("Scheduled backup submitted",
			"component", "daemon",
			"target", targetCopy.Name,
			"job_id", jobID)
	}

	// Add job to scheduler
	_, err := d.scheduler.AddFunc(target.Schedule, job)
	if err != nil {
		return fmt.Errorf("invalid cron schedule '%s': %w", target.Schedule, err)
	}

	return nil
}

// GetJobManager returns the job manager (useful for API access)
func (d *Daemon) GetJobManager() *jobs.Manager {
	return d.jobManager
}

// sendScheduleFailureNotification sends failure notifications for scheduled backup errors
func (d *Daemon) sendScheduleFailureNotification(ctx context.Context, targetName string, err error) {
	if d.cfg == nil || len(d.cfg.Notifiers) == 0 {
		return
	}

	// Build notification message
	msg := &notify.Message{
		Target:      targetName,
		Operation:   "backup",
		Duration:    0,
		Error:       fmt.Errorf("failed to schedule backup: %w", err),
		Timestamp:   time.Now(),
		Manual:      false,
		ScheduledBy: "cron",
	}

	logger := util.GetLogger()
	for notifierName, notifierCfg := range d.cfg.Notifiers {
		notifier, notifyErr := notify.New(notifierCfg)
		if notifyErr != nil {
			logger.WarnS("Failed to create notifier",
				"component", "daemon",
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"error", notifyErr)
			continue
		}

		start := time.Now()
		logger.InfoS("Sending scheduled backup failure notification",
			"component", "daemon",
			"operation", "backup",
			"status", "failure",
			"target", targetName,
			"notifier", notifierName,
			"type", notifierCfg.Type)

		if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
			logger.ErrorS("Failed to send failure notification",
				"component", "daemon",
				"operation", "backup",
				"status", "failure",
				"target", targetName,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"duration", time.Since(start),
				"error", sendErr)
		} else {
			logger.InfoS("Failure notification sent",
				"component", "daemon",
				"operation", "backup",
				"status", "failure",
				"target", targetName,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"duration", time.Since(start))
		}
	}
}

// loadRuntimeConfig loads configuration from DB or YAML fallback
func (d *Daemon) loadRuntimeConfig() (*config.Config, configservice.ConfigSource, error) {
	if d.configLoader != nil {
		cfg, source, err := d.configLoader.LoadConfig(d.ctx)
		if err != nil {
			return nil, "", err
		}
		return cfg, source, nil
	}
	// Fallback to original YAML config
	return d.cfg, configservice.SourceYAML, nil
}

// scheduleAllTargets schedules all targets with cron expressions
func (d *Daemon) scheduleAllTargets(cfg *config.Config) int {
	logger := util.GetLogger()
	scheduledCount := 0
	for _, target := range cfg.Targets {
		if target.Schedule != "" {
			if err := d.scheduleTarget(target); err != nil {
				logger.ErrorS("Failed to schedule target",
					"component", "daemon",
					"target", target.Name,
					"error", err)
				continue
			}
			scheduledCount++
			logger.InfoS("Scheduled target",
				"component", "daemon",
				"target", target.Name,
				"cron", target.Schedule)
		}
	}
	return scheduledCount
}

// hotReloadListener listens for reload requests and reloads configuration
func (d *Daemon) hotReloadListener() {
	logger := util.GetLogger()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-d.reloadChan:
			logger.InfoS("Hot reload requested", "component", "daemon")
			if err := d.reloadConfiguration(); err != nil {
				logger.ErrorS("Hot reload failed",
					"component", "daemon",
					"error", err)
			} else {
				logger.InfoS("Hot reload completed successfully", "component", "daemon")
			}
		}
	}
}

// reloadConfiguration reloads configuration and reschedules targets
func (d *Daemon) reloadConfiguration() error {
	logger := util.GetLogger()

	// Load new configuration
	newCfg, source, err := d.loadRuntimeConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.InfoS("Reloaded configuration",
		"component", "daemon",
		"source", source)

	// Stop existing scheduler
	logger.InfoS("Stopping existing scheduler", "component", "daemon")
	ctx := d.scheduler.Stop()
	<-ctx.Done()

	// Create new scheduler
	d.scheduler = cron.New()

	// Reschedule all targets
	scheduledCount := d.scheduleAllTargets(newCfg)
	logger.InfoS("Rescheduled targets",
		"component", "daemon",
		"count", scheduledCount)

	// Start scheduler if there are scheduled targets
	if scheduledCount > 0 {
		d.scheduler.Start()
		logger.InfoS("Scheduler restarted",
			"component", "daemon",
			"scheduled_targets", scheduledCount)
	}

	return nil
}
