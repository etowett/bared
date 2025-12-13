package persistence

import (
	"context"
	"time"

	"bared/internal/jobs"
)

// JobFilter defines criteria for listing jobs
type JobFilter struct {
	TargetName string
	Status     jobs.JobStatus
	Type       jobs.JobType
	Limit      int
	Offset     int
}

// JobStore defines the interface for persistence operations
type JobStore interface {
	// Job Management
	CreateJob(ctx context.Context, job *jobs.Job) error
	UpdateJob(ctx context.Context, job *jobs.Job) error
	GetJob(ctx context.Context, id jobs.JobID) (*jobs.Job, error)
	ListJobs(ctx context.Context, filter JobFilter) ([]*jobs.Job, error)

	// Locking for Cron
	// AcquireLock attempts to acquire a lock. key is generic (e.g., cron:target:ts).
	// Returns true if acquired, false if already held.
	AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, lockName string) error

	// Close closes the connection
	Close() error
}
