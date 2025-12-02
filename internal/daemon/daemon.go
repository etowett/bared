package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"

	"bared/internal/app"
	"bared/internal/config"
)

// Daemon represents the backup daemon
type Daemon struct {
	cfg       *config.Config
	scheduler *cron.Cron
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new daemon instance
func New(cfg *config.Config) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		cfg:       cfg,
		scheduler: cron.New(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the daemon and scheduler
func (d *Daemon) Start() error {
	log.Println("Starting BareD daemon...")

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
	log.Println("Stopping scheduler...")
	ctx := d.scheduler.Stop()

	// Wait for running jobs to complete
	<-ctx.Done()

	// Cancel context
	d.cancel()

	log.Println("Daemon stopped")
	return nil
}

// Reload reloads the configuration and reschedules targets
func (d *Daemon) Reload() error {
	// Load new configuration
	// Note: We'd need to pass the config file path here
	// For now, this is a placeholder
	log.Println("Configuration reload not yet fully implemented")
	return nil
}

// scheduleTarget adds a target to the scheduler
func (d *Daemon) scheduleTarget(target *config.Target) error {
	// Create a closure that captures the target
	targetCopy := target
	job := func() {
		log.Printf("[%s] Starting scheduled backup", targetCopy.Name)

		result, err := app.BackupTarget(d.ctx, d.cfg, targetCopy)
		if err != nil {
			log.Printf("[%s] Scheduled backup failed: %v", targetCopy.Name, err)
			return
		}

		if result.Success {
			log.Printf("[%s] Scheduled backup completed successfully (duration: %v)", targetCopy.Name, result.Duration)
		}
	}

	// Add job to scheduler
	_, err := d.scheduler.AddFunc(target.Schedule, job)
	if err != nil {
		return fmt.Errorf("invalid cron schedule '%s': %w", target.Schedule, err)
	}

	return nil
}
