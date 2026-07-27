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

	// WithResults asks the store to rebuild each row's typed Result from the
	// persisted JSON. It is off by default because that costs a json.Unmarshal
	// per row, and the only caller that reads a listed job's Result is the
	// dashboard rollup — /api/jobs renders none of it. Store implementations
	// that keep results in memory may ignore this.
	WithResults bool
}

// JobStore defines the interface for persistence operations
type JobStore interface {
	// Job Management
	CreateJob(ctx context.Context, job *Job) error
	UpdateJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, id JobID) (*Job, error)
	ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error)

	// Log Management
	SaveJobLogsBatch(ctx context.Context, jobID JobID, entries []LogEntry) error
	GetJobLogs(ctx context.Context, jobID JobID, limit int) ([]LogEntry, error)

	// Locking for Cron
	AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, lockName string) error

	// Close closes the connection
	Close() error
}
