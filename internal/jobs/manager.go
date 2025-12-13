package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"bared/internal/app"
	"bared/internal/config"
	"bared/internal/util"
)

// Manager orchestrates job queue, worker pool, and lifecycle
type Manager struct {
	jobs          map[JobID]*Job
	runningJobs   map[string]*Job // targetName -> Job (duplicate prevention)
	jobQueue      chan *Job
	maxConcurrent int
	maxHistory    int
	shutdown      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex

	cfg   *config.Config
	store JobStore
}

// NewManager creates a new job manager
func NewManager(cfg *config.Config, store JobStore, maxConcurrent, maxHistory int) *Manager {
	return &Manager{
		jobs:          make(map[JobID]*Job),
		runningJobs:   make(map[string]*Job),
		jobQueue:      make(chan *Job, 100), // Buffered queue
		maxConcurrent: maxConcurrent,
		maxHistory:    maxHistory,
		shutdown:      make(chan struct{}),
		cfg:           cfg,
		store:         store,
	}
}

// Start starts the worker pool
func (m *Manager) Start() {
	for i := 0; i < m.maxConcurrent; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}
	log.Printf("Job manager started with %d workers", m.maxConcurrent)
}

// worker is a worker goroutine that processes jobs from the queue
func (m *Manager) worker(id int) {
	defer m.wg.Done()

	util.Debug("Worker %d started", id)

	for {
		select {
		case job := <-m.jobQueue:
			util.Info("Worker %d picked up job %s (%s for target %s)",
				id, job.ID, job.Type, job.TargetName)
			m.executeJob(job)

		case <-m.shutdown:
			util.Debug("Worker %d shutting down", id)
			return
		}
	}
}

// executeJob executes a job with progress tracking
func (m *Manager) executeJob(job *Job) {
	// Mark as running
	job.MarkStarted()

	// Mark as no longer running when done
	defer func() {
		m.mu.Lock()
		delete(m.runningJobs, job.TargetName)
		m.mu.Unlock()
	}()

	// Setup log hook to capture logs for this job
	oldHook := util.GetLogHook()
	util.SetLogHook(func(level util.LogLevel, message string) {
		levelStr := "info"
		switch level {
		case util.DEBUG:
			levelStr = "debug"
		case util.WARN:
			levelStr = "warn"
		case util.ERROR:
			levelStr = "error"
		}
		job.Logs.Write(levelStr, message)
	})
	defer util.SetLogHook(oldHook)

	// Find target config (try restore target first, fall back to regular target)
	target, _, _, err := m.cfg.ResolveRestoreTarget(job.TargetName)
	if err != nil {
		job.MarkFailed(fmt.Errorf("target not found: %w", err))
		util.Error("Job %s failed: %v", job.ID, err)
		return
	}

	// Execute based on job type
	switch job.Type {
	case JobTypeBackup:
		result, err := app.BackupTarget(job.Context(), m.cfg, target, job.Progress)
		if err != nil {
			// Check if cancelled
			if job.GetStatus() == JobStatusCancelling {
				job.MarkCancelled()
				util.Info("Job %s cancelled", job.ID)
			} else {
				job.MarkFailed(err)
				util.Error("Job %s failed: %v", job.ID, err)
			}

			// Update job in store on failure/cancel
			if m.store != nil {
				if err := m.store.UpdateJob(job.Context(), job); err != nil {
					util.Error("Failed to update job %s status: %v", job.ID, err)
				}
			}
			return
		}
		job.MarkCompleted(result)

		// Update job in store on success
		if m.store != nil {
			if err := m.store.UpdateJob(job.Context(), job); err != nil {
				util.Error("Failed to update job %s status: %v", job.ID, err)
			}
		}
		util.Info("Job %s completed successfully", job.ID)

	case JobTypeRestore:
		options := job.RestoreOptions
		if options == nil {
			options = &app.RestoreOptions{}
		}
		result, err := app.RestoreTargetWithOptions(job.Context(), m.cfg, target, job.BackupPath, options, job.Progress)
		if err != nil {
			if job.GetStatus() == JobStatusCancelling {
				job.MarkCancelled()
				util.Info("Job %s cancelled", job.ID)
			} else {
				job.MarkFailed(err)
				util.Error("Job %s failed: %v", job.ID, err)
			}

			// Update job in store on failure/cancel
			if m.store != nil {
				if err := m.store.UpdateJob(job.Context(), job); err != nil {
					util.Error("Failed to update job %s status: %v", job.ID, err)
				}
			}
			return
		}
		job.MarkCompleted(result)

		// Update job in store on success
		if m.store != nil {
			if err := m.store.UpdateJob(job.Context(), job); err != nil {
				util.Error("Failed to update job %s status: %v", job.ID, err)
			}
		}
		util.Info("Job %s completed successfully", job.ID)

	default:
		job.MarkFailed(fmt.Errorf("unknown job type: %s", job.Type))
	}
}

// SubmitBackup submits a backup job
func (m *Manager) SubmitBackup(ctx context.Context, target *config.Target, manual bool) (JobID, error) {
	// Check if target is already running
	if m.IsTargetRunning(target.Name) {
		return "", fmt.Errorf("target '%s' backup already in progress", target.Name)
	}

	// Create job
	job := NewJob(JobTypeBackup, target.Name, manual)

	// Store job
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.runningJobs[target.Name] = job
	m.mu.Unlock()

	// Persist job
	if m.store != nil {
		if err := m.store.CreateJob(ctx, job); err != nil {
			util.Error("Failed to persist job %s: %v", job.ID, err)
			// We log but continue, as in-memory tracking is primary fallback
		}
	}

	// Queue job
	select {
	case m.jobQueue <- job:
		util.Info("Backup job %s queued for target %s", job.ID, target.Name)
		return job.ID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// SubmitRestore submits a restore job (backward compatible)
func (m *Manager) SubmitRestore(ctx context.Context, target *config.Target, backupPath string, manual bool) (JobID, error) {
	return m.SubmitRestoreWithOptions(ctx, target, backupPath, manual, &app.RestoreOptions{})
}

// SubmitRestoreWithOptions submits a restore job with options
func (m *Manager) SubmitRestoreWithOptions(ctx context.Context, target *config.Target, backupPath string, manual bool, options *app.RestoreOptions) (JobID, error) {
	// Check if target is already running
	if m.IsTargetRunning(target.Name) {
		return "", fmt.Errorf("target '%s' operation already in progress", target.Name)
	}

	// Create job
	job := NewJob(JobTypeRestore, target.Name, manual)
	job.BackupPath = backupPath
	job.RestoreOptions = options

	// Store job
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.runningJobs[target.Name] = job
	m.mu.Unlock()

	// Persist job
	if m.store != nil {
		if err := m.store.CreateJob(ctx, job); err != nil {
			util.Error("Failed to persist job %s: %v", job.ID, err)
		}
	}

	// Queue job
	select {
	case m.jobQueue <- job:
		dryRunSuffix := ""
		if options != nil && options.DryRun {
			dryRunSuffix = " (dry-run)"
		}
		util.Info("Restore job %s queued for target %s%s", job.ID, target.Name, dryRunSuffix)
		return job.ID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// GetJob returns a job by ID
func (m *Manager) GetJob(jobID JobID) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		// Fallback to store if not in memory (e.g. restart)
		// NOTE: This would require loading result/logs/etc from DB which might be partial
		// For now, we only support in-memory + write-behind persistence.
		// Reading from DB for historical jobs is future work or API driven.
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return job, nil
}

// ListJobs returns all jobs
func (m *Manager) ListJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// ListJobsForTarget returns jobs for a specific target
func (m *Manager) ListJobsForTarget(targetName string) []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []*Job
	for _, job := range m.jobs {
		if job.TargetName == targetName {
			jobs = append(jobs, job)
		}
	}

	return jobs
}

// CancelJob requests cancellation of a job
func (m *Manager) CancelJob(jobID JobID) error {
	job, err := m.GetJob(jobID)
	if err != nil {
		return err
	}

	status := job.GetStatus()
	if status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled {
		return fmt.Errorf("cannot cancel job in status: %s", status)
	}

	job.Cancel()
	util.Info("Cancellation requested for job %s", jobID)
	return nil
}

// IsTargetRunning checks if a target has a running job
func (m *Manager) IsTargetRunning(targetName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.runningJobs[targetName]
	return exists
}

// Shutdown gracefully shuts down the job manager
func (m *Manager) Shutdown(ctx context.Context) error {
	log.Println("Shutting down job manager...")

	// Stop accepting new jobs
	close(m.shutdown)

	// Wait for workers to finish or timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers finished gracefully")
		return nil
	case <-ctx.Done():
		log.Println("Job manager shutdown timed out, some jobs may have been terminated")
		return ctx.Err()
	}
}

// CleanupOldJobs removes completed jobs older than the specified duration
func (m *Manager) CleanupOldJobs(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, job := range m.jobs {
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > maxAge {
			delete(m.jobs, id)
			util.Debug("Cleaned up old job %s", id)
		}
	}
}

// StartCleanupRoutine starts a background routine to cleanup old jobs
func (m *Manager) StartCleanupRoutine(interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.CleanupOldJobs(maxAge)
			case <-m.shutdown:
				return
			}
		}
	}()
}
