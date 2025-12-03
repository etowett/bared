package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"bared/internal/api"
	"bared/internal/config"
	"bared/internal/jobs"
)

// Daemon represents the backup daemon
type Daemon struct {
	cfg        *config.Config
	scheduler  *cron.Cron
	jobManager *jobs.Manager
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
	ctx, cancel := context.WithCancel(context.Background())

	// Create daemon with defaults
	d := &Daemon{
		cfg:               cfg,
		scheduler:         cron.New(),
		ctx:               ctx,
		cancel:            cancel,
		maxConcurrentJobs: 3,
		jobHistorySize:    10,
		shutdownTimeout:   1 * time.Hour,
	}

	// Apply options
	for _, opt := range opts {
		opt(d)
	}

	// Create job manager
	d.jobManager = jobs.NewManager(cfg, d.maxConcurrentJobs, d.jobHistorySize)

	// Initialize HTTP server if configured
	if d.httpAddr != "" {
		d.apiServer = api.NewServer(d.httpAddr, d.authUser, d.authPass, d.jobManager, cfg)
	}

	return d
}

// Start starts the daemon and scheduler
func (d *Daemon) Start() error {
	log.Println("Starting BareD daemon...")

	// Start job manager worker pool
	d.jobManager.Start()

	// Start cleanup routine for old jobs (every hour, remove jobs older than 24h)
	d.jobManager.StartCleanupRoutine(1*time.Hour, 24*time.Hour)

	// Start HTTP server if configured
	if d.apiServer != nil {
		go func() {
			if err := d.apiServer.Start(); err != nil {
				log.Printf("HTTP server error: %v", err)
			}
		}()
		log.Printf("HTTP server started on %s", d.httpAddr)
	}

	// Schedule all targets that have a schedule configured
	scheduledCount := 0
	for _, target := range d.cfg.Targets {
		if target.Schedule != "" {
			if err := d.scheduleTarget(target); err != nil {
				return fmt.Errorf("failed to schedule target '%s': %w", target.Name, err)
			}
			scheduledCount++
			log.Printf("Scheduled target '%s' with cron: %s", target.Name, target.Schedule)
		}
	}

	if scheduledCount == 0 {
		return fmt.Errorf("no targets with schedules configured")
	}

	// Start the scheduler
	d.scheduler.Start()
	log.Printf("Scheduler started with %d scheduled targets", scheduledCount)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for signals
	for {
		sig := <-sigChan

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Printf("Received %v signal, shutting down gracefully...", sig)
			return d.Stop()

		case syscall.SIGHUP:
			log.Println("Received SIGHUP signal, reloading configuration...")
			if err := d.Reload(); err != nil {
				log.Printf("Failed to reload configuration: %v", err)
			}
		}
	}
}

// Stop stops the daemon gracefully
func (d *Daemon) Stop() error {
	log.Println("Stopping daemon gracefully...")

	// 1. Stop scheduler (wait for running cron jobs to complete)
	log.Println("Stopping scheduler...")
	ctx := d.scheduler.Stop()
	<-ctx.Done()
	log.Println("Scheduler stopped")

	// 2. Shutdown HTTP server
	if d.apiServer != nil {
		log.Println("Stopping HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.apiServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		log.Println("HTTP server stopped")
	}

	// 3. Shutdown job manager (wait for running jobs with timeout)
	log.Printf("Waiting for running jobs to complete (timeout: %v)...", d.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), d.shutdownTimeout)
	defer cancel()

	if err := d.jobManager.Shutdown(shutdownCtx); err != nil {
		log.Printf("Warning: Job manager shutdown timed out: %v", err)
		log.Println("Some jobs may have been terminated")
	} else {
		log.Println("All jobs completed successfully")
	}

	// 4. Cancel daemon context
	d.cancel()

	log.Println("Daemon stopped")
	return nil
}

// Reload reloads the configuration and reschedules targets
func (d *Daemon) Reload() error {
	// TODO: Implement configuration reload
	// This would involve:
	// 1. Loading new configuration
	// 2. Stopping scheduler
	// 3. Rescheduling targets with new config
	// 4. Starting scheduler
	log.Println("Configuration reload not yet fully implemented")
	return nil
}

// scheduleTarget adds a target to the scheduler
func (d *Daemon) scheduleTarget(target *config.Target) error {
	// Create a closure that captures the target
	targetCopy := target
	job := func() {
		log.Printf("[%s] Starting scheduled backup", targetCopy.Name)

		// Submit job to job manager
		jobID, err := d.jobManager.SubmitBackup(d.ctx, targetCopy, false)
		if err != nil {
			log.Printf("[%s] Failed to submit scheduled backup: %v", targetCopy.Name, err)
			return
		}

		log.Printf("[%s] Scheduled backup submitted as job %s", targetCopy.Name, jobID)
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
