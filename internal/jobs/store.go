package jobs

import (
	"context"
	"time"
)

// JobFilter defines criteria for listing jobs
type JobFilter struct {
	TargetName string
	Status     JobStatus
	Type       JobType
	Limit      int
	Offset     int
}

// JobStore defines the interface for persistence operations
type JobStore interface {
	// Job Management
	CreateJob(ctx context.Context, job *Job) error
	UpdateJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, id JobID) (*Job, error)
	ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error)

	// Locking for Cron
	AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, lockName string) error

	// Close closes the connection
	Close() error
}
