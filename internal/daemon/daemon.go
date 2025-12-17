// Package daemon implements the background process for scheduling and running jobs.
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"bared/internal/api"
	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/notify"
	"bared/internal/persistence"
	"bared/internal/util"
)

// Daemon represents the backup daemon
type Daemon struct {
	cfg        *config.Config
	scheduler  *cron.Cron
	jobManager *jobs.Manager
	store      jobs.JobStore
	ctx        context.Context
	cancel     context.CancelFunc

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
		if err != nil {
			logger.WarnS("Failed to initialize persistence. Running in-memory only.",
				"component", "daemon",
				"driver", driver,
				"dsn", dsn,
				"error", err)
			store = nil // Explicitly set to nil to avoid nil pointer in interface
		} else {
			logger.InfoS("Persistence enabled",
				"component", "daemon",
				"driver", driver,
				"dsn", dsn)
			store = sqlStore // Only assign if successful
		}
	}

	// Create daemon with defaults
	d := &Daemon{
		cfg:               cfg,
		scheduler:         cron.New(),
		ctx:               ctx,
		cancel:            cancel,
		store:             store,
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
		d.apiServer = api.NewServer(d.httpAddr, d.authUser, d.authPass, d.jobManager, cfg)
	}

	return d
}

// Start starts the daemon and scheduler
func (d *Daemon) Start() error {
	logger := util.GetLogger()
	logger.InfoS("Starting BareD daemon", "component", "daemon")

	// Start job manager worker pool
	d.jobManager.Start()

	// Start cleanup routine for old jobs (every hour, remove jobs older than 24h)
	d.jobManager.StartCleanupRoutine(1*time.Hour, 24*time.Hour)

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

	// Schedule all targets that have a schedule configured
	scheduledCount := 0
	for _, target := range d.cfg.Targets {
		if target.Schedule != "" {
			if err := d.scheduleTarget(target); err != nil {
				return fmt.Errorf("failed to schedule target '%s': %w", target.Name, err)
			}
			scheduledCount++
			logger.InfoS("Scheduled target",
				"component", "daemon",
				"target", target.Name,
				"cron", target.Schedule)
		}
	}

	if scheduledCount == 0 {
		return fmt.Errorf("no targets with schedules configured")
	}

	// Start the scheduler
	d.scheduler.Start()
	logger.InfoS("Scheduler started",
		"component", "daemon",
		"scheduled_targets", scheduledCount)

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
