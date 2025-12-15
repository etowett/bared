// Package jobs provides job management and tracking for backup and restore operations.
package jobs

import (
	"context"
	"sync"
	"time"

	"bared/internal/app"
)

// JobID is a unique identifier for a job
type JobID string

// JobStatus represents the current state of a job
type JobStatus string

// Job statuses for tracking operation progress.
const (
	// JobStatusQueued indicates the job is waiting to be executed.
	JobStatusQueued     JobStatus = "queued"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelling JobStatus = "cancelling"
	JobStatusCancelled  JobStatus = "cancelled"
)

// JobType represents the type of operation
type JobType string

// Job types for different operations.
const (
	// JobTypeBackup indicates a backup operation.
	JobTypeBackup  JobType = "backup"
	JobTypeRestore JobType = "restore"
)

// Job represents a backup or restore operation
type Job struct {
	ID             JobID
	Type           JobType
	TargetName     string
	Status         JobStatus
	Progress       *Progress
	Result         interface{} // *app.BackupResult or *app.RestoreResult
	Error          string
	CreatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Logs           *LogBuffer
	BackupPath     string              // For restore jobs
	RestoreOptions *app.RestoreOptions // For restore jobs with options
	Manual         bool                // true if triggered manually via API
	ScheduledBy    string              // cron schedule that triggered this

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// NewJob creates a new job instance
func NewJob(jobType JobType, targetName string, manual bool) *Job {
	ctx, cancel := context.WithCancel(context.Background())

	return &Job{
		ID:         JobID(generateJobID()),
		Type:       jobType,
		TargetName: targetName,
		Status:     JobStatusQueued,
		Progress:   NewProgress(),
		CreatedAt:  time.Now(),
		Logs:       NewLogBuffer(1000), // 1000 log entries max
		Manual:     manual,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Context returns the job's context
func (j *Job) Context() context.Context {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.ctx
}

// Cancel requests cancellation of the job
func (j *Job) Cancel() {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.Status == JobStatusQueued || j.Status == JobStatusRunning {
		j.Status = JobStatusCancelling
		j.cancel()
	}
}

// SetStatus updates the job status thread-safely
func (j *Job) SetStatus(status JobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
}

// GetStatus returns the current job status
func (j *Job) GetStatus() JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Status
}

// MarkStarted marks the job as started
func (j *Job) MarkStarted() {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	j.StartedAt = &now
	j.Status = JobStatusRunning
}

// MarkCompleted marks the job as completed successfully
func (j *Job) MarkCompleted(result interface{}) {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	j.CompletedAt = &now
	j.Status = JobStatusCompleted
	j.Result = result
}

// MarkFailed marks the job as failed
func (j *Job) MarkFailed(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	j.CompletedAt = &now
	j.Status = JobStatusFailed
	j.Error = err.Error()
}

// MarkCancelled marks the job as cancelled
func (j *Job) MarkCancelled() {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	j.CompletedAt = &now
	j.Status = JobStatusCancelled
}

// Progress represents the progress state of a job
type Progress struct {
	Stage          string
	Percent        float64
	BytesProcessed int64
	BytesTotal     int64
	StartTime      time.Time
	ETA            *time.Time
	Message        string

	// EMA tracking for smooth ETA
	bytesPerSecEMA float64
	lastUpdateTime time.Time
	lastBytes      int64

	mu sync.RWMutex
}

// NewProgress creates a new progress tracker
func NewProgress() *Progress {
	return &Progress{
		Stage:          "initializing",
		Percent:        0,
		StartTime:      time.Now(),
		lastUpdateTime: time.Now(),
	}
}

// SetStage sets the current stage and estimated bytes
func (p *Progress) SetStage(stage string, estimatedBytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Stage = stage
	p.BytesTotal = estimatedBytes
	p.BytesProcessed = 0
	p.Message = ""
}

// Update updates the progress percentage and message
func (p *Progress) Update(percent float64, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Percent = percent
	p.Message = message
}

// UpdateBytes updates bytes processed and recalculates progress
func (p *Progress) UpdateBytes(processed, total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.BytesProcessed = processed
	if total > 0 {
		p.BytesTotal = total
	}

	// Calculate percentage if we know the total
	if p.BytesTotal > 0 {
		p.Percent = float64(p.BytesProcessed) / float64(p.BytesTotal) * 100
		if p.Percent > 100 {
			p.Percent = 100
		}
	}

	// Update ETA
	p.calculateETA()
}

// calculateETA calculates the estimated time of completion using EMA
func (p *Progress) calculateETA() {
	now := time.Now()
	elapsed := now.Sub(p.lastUpdateTime).Seconds()

	// Only calculate if we have meaningful data
	if elapsed < 0.1 || p.BytesTotal == 0 || p.Percent < 10 {
		return
	}

	// Calculate current rate
	bytesDelta := p.BytesProcessed - p.lastBytes
	currentRate := float64(bytesDelta) / elapsed

	// Update EMA (alpha = 0.3 for balance between recency and smoothing)
	const alpha = 0.3
	if p.bytesPerSecEMA == 0 {
		p.bytesPerSecEMA = currentRate
	} else {
		p.bytesPerSecEMA = alpha*currentRate + (1-alpha)*p.bytesPerSecEMA
	}

	// Calculate ETA
	if p.bytesPerSecEMA > 0 {
		remaining := p.BytesTotal - p.BytesProcessed
		secondsRemaining := float64(remaining) / p.bytesPerSecEMA
		eta := now.Add(time.Duration(secondsRemaining) * time.Second)
		p.ETA = &eta
	}

	// Update tracking variables
	p.lastUpdateTime = now
	p.lastBytes = p.BytesProcessed
}

// GetSnapshot returns a thread-safe copy of the current progress
func (p *Progress) GetSnapshot() ProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProgressSnapshot{
		Stage:          p.Stage,
		Percent:        p.Percent,
		BytesProcessed: p.BytesProcessed,
		BytesTotal:     p.BytesTotal,
		ETA:            p.ETA,
		Message:        p.Message,
	}
}

// ProgressSnapshot is a thread-safe snapshot of progress state
type ProgressSnapshot struct {
	Stage          string
	Percent        float64
	BytesProcessed int64
	BytesTotal     int64
	ETA            *time.Time
	Message        string
}

// generateJobID generates a unique job ID
func generateJobID() string {
	// Use timestamp + random component for uniqueness
	return time.Now().Format("20060102-150405") + "-" + randomString(8)
}

// randomString generates a random alphanumeric string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1 * time.Nanosecond) // Ensure uniqueness
	}
	return string(b)
}
