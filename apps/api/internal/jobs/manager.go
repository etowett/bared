package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/etowett/bared/apps/api/internal/app"
	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/configservice"
	"github.com/etowett/bared/apps/api/internal/notify"
	"github.com/etowett/bared/apps/api/internal/util"
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

	cfg           *config.Config
	store         JobStore
	configService *configservice.Service

	// startedAt is when this manager was created. Without a store it is the
	// horizon of everything the daemon knows: the in-memory map starts empty,
	// so nothing before it can be counted. See CoversWindow.
	startedAt time.Time
}

// NewManager creates a new job manager
func NewManager(cfg *config.Config, store JobStore, configService *configservice.Service, maxConcurrent, maxHistory int) *Manager {
	return &Manager{
		jobs:          make(map[JobID]*Job),
		runningJobs:   make(map[string]*Job),
		jobQueue:      make(chan *Job, 100), // Buffered queue
		maxConcurrent: maxConcurrent,
		maxHistory:    maxHistory,
		shutdown:      make(chan struct{}),
		cfg:           cfg,
		store:         store,
		configService: configService,
		startedAt:     time.Now(),
	}
}

// Start starts the worker pool
func (m *Manager) Start() {
	for i := 0; i < m.maxConcurrent; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}
	util.GetLogger().InfoS("Job manager started",
		"component", "job_manager",
		"workers", m.maxConcurrent)
}

// worker is a worker goroutine that processes jobs from the queue
func (m *Manager) worker(id int) {
	defer m.wg.Done()

	logger := util.GetLogger()
	logger.InfoS("Worker started",
		"component", "job_manager",
		"worker_id", id)

	for {
		select {
		case job := <-m.jobQueue:
			logger.InfoS("Worker picked up job",
				"component", "job_manager",
				"worker_id", id,
				"job_id", job.ID,
				"job_type", job.Type,
				"target", job.TargetName)
			m.executeJob(job)

		case <-m.shutdown:
			logger.InfoS("Worker shutting down",
				"component", "job_manager",
				"worker_id", id)
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

	// Setup log persistence batch flusher if store is available
	var logFlushDone chan struct{}
	var logBuffer []LogEntry
	var logBufferMu sync.Mutex
	var flushWg sync.WaitGroup

	if m.store != nil {
		logFlushDone = make(chan struct{})
		ticker := time.NewTicker(5 * time.Second)

		// Start periodic log flusher
		flushWg.Add(1)
		go func() {
			defer flushWg.Done()
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					// Periodic flush (every 5 seconds)
					logBufferMu.Lock()
					if len(logBuffer) > 0 {
						entries := make([]LogEntry, len(logBuffer))
						copy(entries, logBuffer)
						logBuffer = logBuffer[:0]
						logBufferMu.Unlock()

						if err := m.store.SaveJobLogsBatch(context.Background(), job.ID, entries); err != nil {
							util.GetLogger().ErrorS("Failed to persist log entries",
								"component", "job_manager",
								"job_id", job.ID,
								"entry_count", len(entries),
								"error", err)
						}
					} else {
						logBufferMu.Unlock()
					}

				case <-logFlushDone:
					// Final flush on job completion
					logBufferMu.Lock()
					if len(logBuffer) > 0 {
						if err := m.store.SaveJobLogsBatch(context.Background(), job.ID, logBuffer); err != nil {
							util.GetLogger().ErrorS("Failed to persist final log entries",
								"component", "job_manager",
								"job_id", job.ID,
								"entry_count", len(logBuffer),
								"error", err)
						}
					}
					logBufferMu.Unlock()
					return
				}
			}
		}()
	}

	// Defer final log flush
	defer func() {
		if logFlushDone != nil {
			close(logFlushDone)
			flushWg.Wait() // Properly wait for flush to complete
		}
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

		// Add to batch buffer for persistence
		if m.store != nil {
			logBufferMu.Lock()
			logBuffer = append(logBuffer, LogEntry{
				Timestamp: time.Now(),
				Level:     levelStr,
				Message:   message,
			})
			// Immediate flush if buffer reaches 100 entries
			if len(logBuffer) >= 100 {
				entries := make([]LogEntry, len(logBuffer))
				copy(entries, logBuffer)
				logBuffer = logBuffer[:0]
				logBufferMu.Unlock()

				go func() {
					if err := m.store.SaveJobLogsBatch(context.Background(), job.ID, entries); err != nil {
						util.GetLogger().ErrorS("Failed to persist batch of log entries",
							"component", "job_manager",
							"job_id", job.ID,
							"entry_count", len(entries),
							"error", err)
					}
				}()
			} else {
				logBufferMu.Unlock()
			}
		}
	})
	defer util.SetLogHook(oldHook)

	// Find target config - try database first, fall back to static config
	target, cfg, err := m.resolveTargetWithStorage(job.Context(), job.TargetName)
	if err != nil {
		targetErr := fmt.Errorf("target not found: %w", err)
		job.MarkFailed(targetErr)
		util.GetLogger().ErrorS("Job failed - target not found",
			"component", "job_manager",
			"job_id", job.ID,
			"target", job.TargetName,
			"error", err)

		// Send failure notification
		operation := "backup"
		if job.Type == JobTypeRestore {
			operation = "restore"
		}
		m.sendFailureNotification(job.Context(), job, targetErr, operation)
		return
	}

	// Execute based on job type
	switch job.Type {
	case JobTypeBackup:
		result, err := app.BackupTarget(job.Context(), cfg, target, job.Progress)
		if err != nil {
			logger := util.GetLogger()
			// Check if cancelled
			if job.GetStatus() == JobStatusCancelling {
				job.MarkCancelled()
				logger.InfoS("Job cancelled",
					"component", "job_manager",
					"job_id", job.ID,
					"job_type", "backup",
					"target", job.TargetName)
			} else {
				wrappedErr := fmt.Errorf("backup failed: %w", err)
				job.MarkFailed(wrappedErr)
				logger.ErrorS("Job failed",
					"component", "job_manager",
					"job_id", job.ID,
					"job_type", "backup",
					"target", job.TargetName,
					"error", util.RedactErr(wrappedErr))

				// Send failure notification
				m.sendFailureNotification(job.Context(), job, wrappedErr, "backup")
			}

			// Update job in store on failure/cancel
			if m.store != nil {
				if err := m.store.UpdateJob(job.Context(), job); err != nil {
					logger.ErrorS("Failed to update job status",
						"component", "job_manager",
						"job_id", job.ID,
						"error", err)
				}
			}
			return
		}
		job.MarkCompleted(result)

		// Update job in store on success
		logger := util.GetLogger()
		if m.store != nil {
			if err := m.store.UpdateJob(job.Context(), job); err != nil {
				logger.ErrorS("Failed to update job status",
					"component", "job_manager",
					"job_id", job.ID,
					"error", err)
			}
		}
		logger.InfoS("Job completed successfully",
			"component", "job_manager",
			"job_id", job.ID,
			"job_type", "backup",
			"target", job.TargetName)

	case JobTypeRestore:
		options := job.RestoreOptions
		if options == nil {
			options = &app.RestoreOptions{}
		}
		result, err := app.RestoreTargetWithOptions(job.Context(), cfg, target, job.BackupPath, options, job.Progress)
		if err != nil {
			logger := util.GetLogger()
			if job.GetStatus() == JobStatusCancelling {
				job.MarkCancelled()
				logger.InfoS("Job cancelled",
					"component", "job_manager",
					"job_id", job.ID,
					"job_type", "restore",
					"target", job.TargetName)
			} else {
				wrappedErr := fmt.Errorf("restore failed: %w", err)
				job.MarkFailed(wrappedErr)
				logger.ErrorS("Job failed",
					"component", "job_manager",
					"job_id", job.ID,
					"job_type", "restore",
					"target", job.TargetName,
					"error", util.RedactErr(wrappedErr))

				// Send failure notification
				m.sendFailureNotification(job.Context(), job, wrappedErr, "restore")
			}

			// Update job in store on failure/cancel
			if m.store != nil {
				if err := m.store.UpdateJob(job.Context(), job); err != nil {
					logger.ErrorS("Failed to update job status",
						"component", "job_manager",
						"job_id", job.ID,
						"error", err)
				}
			}
			return
		}
		job.MarkCompleted(result)

		// Update job in store on success
		logger := util.GetLogger()
		if m.store != nil {
			if err := m.store.UpdateJob(job.Context(), job); err != nil {
				logger.ErrorS("Failed to update job status",
					"component", "job_manager",
					"job_id", job.ID,
					"error", err)
			}
		}
		logger.InfoS("Job completed successfully",
			"component", "job_manager",
			"job_id", job.ID,
			"job_type", "restore",
			"target", job.TargetName)

	default:
		unknownErr := fmt.Errorf("unknown job type: %s", job.Type)
		job.MarkFailed(unknownErr)
		util.GetLogger().ErrorS("Job failed - unknown job type",
			"component", "job_manager",
			"job_id", job.ID,
			"job_type", job.Type,
			"target", job.TargetName,
			"error", unknownErr)

		// Send failure notification
		m.sendFailureNotification(job.Context(), job, unknownErr, string(job.Type))
	}
}

// SubmitBackup submits a backup job
func (m *Manager) SubmitBackup(ctx context.Context, target *config.Target, manual bool) (JobID, error) {
	logger := util.GetLogger()
	logger.InfoS("Submitting backup job",
		"component", "job_manager",
		"target", target.Name)

	// Check if target is already running
	if m.IsTargetRunning(target.Name) {
		return "", fmt.Errorf("target '%s' backup already in progress", target.Name)
	}

	// Create job
	job := NewJob(JobTypeBackup, target.Name, manual)

	logger.InfoS("Created job",
		"component", "job_manager",
		"job_id", job.ID,
		"job_type", job.Type,
		"target", target.Name,
		"manual", manual)

	// Store job
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.runningJobs[target.Name] = job
	m.mu.Unlock()

	// Persist job
	if m.store != nil {
		if err := m.store.CreateJob(ctx, job); err != nil {
			logger.ErrorS("Failed to persist job",
				"component", "job_manager",
				"job_id", job.ID,
				"error", err)
			// We log but continue, as in-memory tracking is primary fallback
		}
	}

	// Queue job
	select {
	case m.jobQueue <- job:
		logger.InfoS("Backup job queued",
			"component", "job_manager",
			"job_id", job.ID,
			"target", target.Name)
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
	logger := util.GetLogger()
	if m.store != nil {
		if err := m.store.CreateJob(ctx, job); err != nil {
			logger.ErrorS("Failed to persist job",
				"component", "job_manager",
				"job_id", job.ID,
				"error", err)
		}
	}

	// Queue job
	select {
	case m.jobQueue <- job:
		dryRun := options != nil && options.DryRun
		logger.InfoS("Restore job queued",
			"component", "job_manager",
			"job_id", job.ID,
			"target", target.Name,
			"dry_run", dryRun)
		return job.ID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// GetJob returns a job by ID
func (m *Manager) GetJob(jobID JobID) (*Job, error) {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		// Fallback to store if not in memory (e.g. after restart)
		if m.store != nil {
			storedJob, err := m.store.GetJob(context.Background(), jobID)
			if err != nil {
				return nil, fmt.Errorf("job not found: %s", jobID)
			}

			// Load historical logs from database
			logs, err := m.store.GetJobLogs(context.Background(), jobID, 1000)
			if err == nil && len(logs) > 0 {
				// Populate log buffer with historical logs
				storedJob.Logs = NewLogBuffer(1000)
				for _, entry := range logs {
					storedJob.Logs.WriteWithStageAndTimestamp(entry.Level, entry.Message, entry.Stage, entry.Timestamp)
				}
			}

			return storedJob, nil
		}
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return job, nil
}

// resolveTargetWithStorage resolves a target from DB or config and ensures its storage/notifier references are populated
func (m *Manager) resolveTargetWithStorage(ctx context.Context, targetName string) (*config.Target, *config.Config, error) {
	// Try to get target from configService first (database)
	if m.configService != nil {
		target, err := m.configService.GetTarget(ctx, targetName)
		if err == nil {
			// Target found in DB, now resolve storage reference
			if target.Storage != nil && target.Storage.Enabled && target.Storage.Name != "" {
				storage, err := m.configService.GetStorage(ctx, target.Storage.Name)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to resolve storage '%s' for target '%s': %w", target.Storage.Name, targetName, err)
				}
				// Replace storage reference with full storage config
				target.Storage = &config.TargetStorage{
					Enabled: true,
					Name:    storage.Name,
				}

				// Create a temporary config with the resolved storage and notifiers
				tempCfg := &config.Config{
					Storages: map[string]*config.Storage{
						storage.Name: storage,
					},
					Targets: []*config.Target{target},
				}

				// Get default storage if set
				if defaultStorage, err := m.configService.GetGlobalConfig(ctx, "default_storage"); err == nil && defaultStorage != "" {
					tempCfg.DefaultStorage = defaultStorage
					// Ensure default storage is in the map
					if _, exists := tempCfg.Storages[defaultStorage]; !exists {
						if ds, err := m.configService.GetStorage(ctx, defaultStorage); err == nil {
							tempCfg.Storages[defaultStorage] = ds
						}
					}
				}

				// Get notifiers
				notifiers, err := m.configService.ListNotifiers(ctx)
				if err == nil {
					tempCfg.Notifiers = notifiers
				}

				return target, tempCfg, nil
			}

			// No storage reference, try to use default storage
			defaultStorage, err := m.configService.GetGlobalConfig(ctx, "default_storage")
			if err == nil && defaultStorage != "" {
				storage, err := m.configService.GetStorage(ctx, defaultStorage)
				if err == nil {
					tempCfg := &config.Config{
						DefaultStorage: defaultStorage,
						Storages: map[string]*config.Storage{
							storage.Name: storage,
						},
						Targets: []*config.Target{target},
					}

					// Get notifiers
					notifiers, err := m.configService.ListNotifiers(ctx)
					if err == nil {
						tempCfg.Notifiers = notifiers
					}

					return target, tempCfg, nil
				}
			}

			return target, m.cfg, nil
		}

		// Try restore target from DB
		restoreTarget, err := m.configService.GetRestoreTarget(ctx, targetName)
		if err == nil {
			// Convert to regular target for restore operations
			target := &config.Target{
				Name:    restoreTarget.Name,
				Conn:    restoreTarget.Conn,
				Storage: restoreTarget.Storage,
			}

			// Resolve storage if present
			if restoreTarget.Storage != nil && restoreTarget.Storage.Enabled && restoreTarget.Storage.Name != "" {
				storage, err := m.configService.GetStorage(ctx, restoreTarget.Storage.Name)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to resolve storage '%s' for restore target '%s': %w", restoreTarget.Storage.Name, targetName, err)
				}

				tempCfg := &config.Config{
					Storages: map[string]*config.Storage{
						storage.Name: storage,
					},
					Targets:        []*config.Target{target},
					RestoreTargets: []*config.RestoreTarget{restoreTarget},
				}

				// Get notifiers
				notifiers, err := m.configService.ListNotifiers(ctx)
				if err == nil {
					tempCfg.Notifiers = notifiers
				}

				return target, tempCfg, nil
			}

			return target, m.cfg, nil
		}
	}

	// Fall back to static config
	target, _, _, err := m.cfg.ResolveRestoreTarget(targetName)
	if err != nil {
		return nil, nil, fmt.Errorf("target not found: %w", err)
	}

	return target, m.cfg, nil
}

func jobMatchesFilter(job *Job, filter JobFilter) bool {
	if filter.TargetName != "" && job.TargetName != filter.TargetName {
		return false
	}
	if filter.Status != "" && job.GetStatus() != filter.Status {
		return false
	}
	if filter.Type != "" && job.Type != filter.Type {
		return false
	}
	return true
}

// ListJobsFiltered returns jobs from both memory and database, filtered and sorted by creation time (newest first).
//
// Semantics:
//   - Memory is the primary source of truth; DB is merged in as a fallback/history source.
//   - Memory jobs take precedence over DB jobs with the same JobID.
//   - If filter.Limit is 0, no global limit is applied (legacy behavior), but DB fetch is capped (default 1000).
//   - If filter.Limit > 0, limit/offset are applied AFTER merging+sorting (global pagination),
//     and the DB fetch limit is increased to reduce the chance of under-filling due to de-duplication.
//
// A store read failure is logged and the in-memory jobs are returned on their
// own. Callers that summarise history — where a short answer and a complete one
// look identical — must use ListJobsFilteredE instead and handle the error.
func (m *Manager) ListJobsFiltered(ctx context.Context, filter JobFilter) []*Job {
	list, err := m.ListJobsFilteredE(ctx, filter)
	if err != nil {
		util.GetLogger().ErrorS("Failed to fetch jobs from database",
			"component", "job_manager",
			"error", err)
	}
	return list
}

// ListJobsFilteredE is ListJobsFiltered with the store read failure surfaced.
//
// The returned slice is still usable when err is non-nil — it holds the jobs
// still in memory — but it is missing everything that only exists in the store.
// Treat it as a partial sample: reporting "no failures" or "never backed up"
// from it is a claim the daemon cannot support.
func (m *Manager) ListJobsFilteredE(ctx context.Context, filter JobFilter) ([]*Job, error) {
	var storeErr error

	// Snapshot in-memory jobs matching filter
	m.mu.RLock()
	memJobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		if jobMatchesFilter(job, filter) {
			memJobs = append(memJobs, job)
		}
	}
	m.mu.RUnlock()

	jobMap := make(map[JobID]*Job, len(memJobs))
	for _, job := range memJobs {
		jobMap[job.ID] = job
	}

	// Fetch + merge DB jobs (filtered + ordered by DB)
	if m.store != nil {

		dbFilter := filter
		// We apply global offset/limit after merge+sort; fetch from the start and increase limit to compensate.
		dbFilter.Offset = 0
		if dbFilter.Limit <= 0 {
			dbFilter.Limit = 1000
		} else {
			// Try to fetch enough rows to satisfy global pagination even after de-dupe + overlay.
			dbFilter.Limit = dbFilter.Limit + maxInt(0, filter.Offset) + len(memJobs)
		}

		dbJobs, err := m.store.ListJobs(ctx, dbFilter)
		if err != nil {
			// Continue with in-memory jobs, but tell the caller the sample is
			// incomplete so it can refuse to answer rather than answer wrongly.
			storeErr = fmt.Errorf("list jobs from store: %w", err)
		} else {
			for _, dbJob := range dbJobs {
				if _, exists := jobMap[dbJob.ID]; !exists {
					jobMap[dbJob.ID] = dbJob
				}
			}
		}
	}

	// Convert map to slice
	jobs := make([]*Job, 0, len(jobMap))
	for _, job := range jobMap {
		jobs = append(jobs, job)
	}

	// Sort by creation time, newest first
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})

	// Apply global pagination (after merge+sort)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(jobs) {
		return []*Job{}, storeErr
	}
	if offset > 0 {
		jobs = jobs[offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(jobs) {
		jobs = jobs[:filter.Limit]
	}
	return jobs, storeErr
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ListJobs returns all jobs from both memory and database (sorted newest first).
func (m *Manager) ListJobs(ctx context.Context) []*Job {
	return m.ListJobsFiltered(ctx, JobFilter{})
}

// ListJobsForTarget returns jobs for a specific target from both memory and database
func (m *Manager) ListJobsForTarget(ctx context.Context, targetName string) []*Job {
	return m.ListJobsFiltered(ctx, JobFilter{TargetName: targetName})
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
	util.GetLogger().InfoS("Cancellation requested for job",
		"component", "job_manager",
		"job_id", jobID)
	return nil
}

// HasPersistence reports whether job history is written to a durable store.
//
// Callers that summarise history over a time window need this: without a store,
// jobs live only in the in-memory map, which CleanupOldJobs prunes. Any window
// longer than that horizon would be answered from a truncated sample.
func (m *Manager) HasPersistence() bool {
	return m.store != nil
}

// CoversWindow reports whether job history spans the whole of the window
// ending at now, and so can be summarised without silent truncation.
//
// With a store, history outlives the process. Without one it begins when the
// manager was created: a daemon restarted twenty minutes ago holds twenty
// minutes of jobs, and a "24h success rate: 100%" computed from them is the
// same lie as one computed from a truncated page.
func (m *Manager) CoversWindow(window time.Duration, now time.Time) bool {
	if m.HasPersistence() {
		return true
	}
	return !m.startedAt.After(now.Add(-window))
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
	logger := util.GetLogger()
	logger.InfoS("Shutting down job manager",
		"component", "job_manager")

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
		logger.InfoS("All workers finished gracefully",
			"component", "job_manager")
		return nil
	case <-ctx.Done():
		logger.WarnS("Job manager shutdown timed out",
			"component", "job_manager")
		return ctx.Err()
	}
}

// CleanupOldJobs removes completed jobs older than the specified duration
func (m *Manager) CleanupOldJobs(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	logger := util.GetLogger()
	for id, job := range m.jobs {
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > maxAge {
			delete(m.jobs, id)
			logger.DebugS("Cleaned up old job",
				"component", "job_manager",
				"job_id", id)
		}
	}
}

// StartCleanupRoutine starts a background routine to cleanup old jobs
func (m *Manager) StartCleanupRoutine(interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger := util.GetLogger()
		logger.InfoS("Cleanup routine started",
			"component", "job_manager",
			"interval", interval,
			"max_age", maxAge)

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

// sendFailureNotification sends failure notifications for job manager errors
func (m *Manager) sendFailureNotification(ctx context.Context, job *Job, err error, operation string) {
	if m.cfg == nil || len(m.cfg.Notifiers) == 0 {
		return
	}

	// Build notification message. The error goes out to Slack/email/webhook, so
	// it gets the same scrubbing as the persisted one (issue #133).
	msg := &notify.Message{
		Target:    job.TargetName,
		Operation: operation,
		Duration:  0, // No duration for early failures
		Error:     util.RedactErr(err),
		Timestamp: time.Now(),
		Manual:    job.Manual,
	}

	// Add schedule info if not manual
	if !job.Manual && job.ScheduledBy != "" {
		msg.ScheduledBy = job.ScheduledBy
	}

	logger := util.GetLogger()
	for notifierName, notifierCfg := range m.cfg.Notifiers {
		notifier, notifyErr := notify.New(notifierCfg)
		if notifyErr != nil {
			logger.WarnS("Failed to create notifier",
				"component", "job_manager",
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"error", notifyErr)
			continue
		}

		start := time.Now()
		logger.InfoS("Sending failure notification",
			"component", "job_manager",
			"operation", operation,
			"status", "failure",
			"target", job.TargetName,
			"notifier", notifierName,
			"type", notifierCfg.Type)

		if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
			logger.ErrorS("Failed to send failure notification",
				"component", "job_manager",
				"operation", operation,
				"status", "failure",
				"target", job.TargetName,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"duration", time.Since(start),
				"error", sendErr)
		} else {
			logger.InfoS("Failure notification sent",
				"component", "job_manager",
				"operation", operation,
				"status", "failure",
				"target", job.TargetName,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"duration", time.Since(start))
		}
	}
}
