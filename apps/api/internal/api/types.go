package api

import (
	"time"

	"github.com/etowett/bared/apps/api/internal/jobs"
)

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// JobResponse represents a job in API responses
type JobResponse struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Target         string            `json:"target"`
	Status         string            `json:"status"`
	Progress       *ProgressResponse `json:"progress,omitempty"`
	Error          string            `json:"error,omitempty"`
	CreatedAt      string            `json:"created_at"`
	StartedAt      *string           `json:"started_at,omitempty"`
	CompletedAt    *string           `json:"completed_at,omitempty"`
	Duration       *float64          `json:"duration_seconds,omitempty"`
	Manual         bool              `json:"manual"`
	BackupPath     string            `json:"backup_path,omitempty"`
	TargetSchedule *string           `json:"target_schedule,omitempty"` // The target's current schedule (cron expression)
	TriggeredBy    *string           `json:"triggered_by,omitempty"`    // "manual", "schedule", or "api"
}

// ProgressResponse represents progress information
type ProgressResponse struct {
	Stage          string  `json:"stage"`
	Percent        float64 `json:"percent"`
	BytesProcessed int64   `json:"bytes_processed"`
	BytesTotal     int64   `json:"bytes_total"`
	ETA            *string `json:"eta,omitempty"`
	Message        string  `json:"message"`
}

// PaginationMetadata represents pagination information
type PaginationMetadata struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// ListJobsResponse represents the response for listing jobs
type ListJobsResponse struct {
	Jobs       []JobResponse      `json:"jobs"`
	Total      int                `json:"total"`
	Pagination PaginationMetadata `json:"pagination"`
}

// TriggerBackupRequest represents a backup trigger request
type TriggerBackupRequest struct {
	Target string `json:"target"`
}

// TriggerRestoreRequest represents a restore trigger request
type TriggerRestoreRequest struct {
	Target     string `json:"target"`
	BackupPath string `json:"backup_path"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

// JobCreatedResponse represents the response after creating a job
type JobCreatedResponse struct {
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}

// TargetSummary represents a target's status.
//
// Everything below IsRunning is derived from backup job history. Fields that
// are pointers are genuinely optional: a nil value means "not known", which is
// not the same as zero. Clients must render an absent value as unknown rather
// than as 0.
type TargetSummary struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Database      string  `json:"database"`
	LastBackup    *string `json:"last_backup,omitempty"`
	NextScheduled *string `json:"next_scheduled,omitempty"`
	Schedule      string  `json:"schedule,omitempty"`
	IsRunning     bool    `json:"is_running"`

	// LastBackupStatus is the outcome of the most recent finished backup job:
	// "success", "failed", or "never" when the target has no backup history.
	// Cancelled jobs are neither, and are ignored.
	LastBackupStatus string `json:"last_backup_status"`

	// ConsecutiveFailures counts failed backup jobs since the last successful
	// one (all of them, when there has never been a success).
	ConsecutiveFailures int `json:"consecutive_failures"`

	// LastBackupBytes is the artifact size recorded by the most recent
	// successful backup. Nil when that job's result was not retained — job
	// rows written before results were persisted carry no size.
	LastBackupBytes *int64 `json:"last_backup_bytes,omitempty"`

	// LastBackupDurationSeconds is how long the most recent successful backup
	// job ran. Nil when the job's timestamps are incomplete.
	LastBackupDurationSeconds *float64 `json:"last_backup_duration_seconds,omitempty"`

	// Overdue is true when the target has a schedule and the run due after its
	// last successful backup has already passed. It stays false for targets
	// with no schedule and for targets with no job history at all, because
	// nothing records when such a target was first configured.
	Overdue bool `json:"overdue"`
}

// ListTargetsResponse represents the response for listing targets
type ListTargetsResponse struct {
	Targets []TargetSummary `json:"targets"`
	Total   int             `json:"total"`
}

// DashboardResponse represents the dashboard summary
type DashboardResponse struct {
	Targets    []TargetSummary `json:"targets"`
	ActiveJobs int             `json:"active_jobs"`
	TotalJobs  int             `json:"total_jobs"`

	// TotalStorage is the number of bytes currently held across storage
	// backends. Nothing tracks that cheaply today — job history records the
	// size of each backup ever taken, including ones retention has since
	// deleted, and listing every backend on each dashboard request would be
	// slow and, for S3, billable. The field is therefore left unset rather
	// than filled with a number that would be wrong; clients must render its
	// absence as "unavailable", never as 0.
	TotalStorage int64 `json:"total_storage_bytes,omitempty"`

	// SuccessRate24h is the percentage (0-100) of backup jobs finishing in the
	// last 24 hours that succeeded. Nil when no backup job finished in the
	// window — a rate over an empty sample is not 0%.
	SuccessRate24h *float64 `json:"success_rate_24h,omitempty"`

	// SuccessRate7d is the same figure over 7 days. It is nil unless job
	// history is persisted: without a store, history lives only in memory and
	// is pruned well inside 7 days, so the sample would be silently truncated.
	SuccessRate7d *float64 `json:"success_rate_7d,omitempty"`

	// FailedJobs24h counts backup jobs that failed in the last 24 hours. Zero
	// is a real answer and is serialized; nil means the count could not be
	// established and is omitted.
	FailedJobs24h *int `json:"failed_jobs_24h,omitempty"`
}

// RestoreTargetSummary represents a restore target's info
type RestoreTargetSummary struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Database     string `json:"database"`
	Host         string `json:"host"`
	Description  string `json:"description,omitempty"`
	SourceTarget string `json:"source_target,omitempty"`
}

// ListRestoreTargetsResponse represents the response for listing restore targets
type ListRestoreTargetsResponse struct {
	RestoreTargets []RestoreTargetSummary `json:"restore_targets"`
	Total          int                    `json:"total"`
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// LogsResponse represents the response for job logs
type LogsResponse struct {
	JobID string     `json:"job_id"`
	Logs  []LogEntry `json:"logs"`
	Total int        `json:"total"`
}

// JobToResponse converts a job to an API response
func JobToResponse(job *jobs.Job) JobResponse {
	resp := JobResponse{
		ID:         string(job.ID),
		Type:       string(job.Type),
		Target:     job.TargetName,
		Status:     string(job.GetStatus()),
		CreatedAt:  job.CreatedAt.Format(time.RFC3339),
		Manual:     job.Manual,
		BackupPath: job.BackupPath,
	}

	// Add started time if available
	if job.StartedAt != nil {
		startedStr := job.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &startedStr
	}

	// Add completed time and duration if available
	if job.CompletedAt != nil {
		completedStr := job.CompletedAt.Format(time.RFC3339)
		resp.CompletedAt = &completedStr

		if job.StartedAt != nil {
			duration := job.CompletedAt.Sub(*job.StartedAt).Seconds()
			resp.Duration = &duration
		}
	}

	// Add error if present
	if job.Error != "" {
		resp.Error = job.Error
	}

	// Add progress if available
	if job.Progress != nil {
		snapshot := job.Progress.GetSnapshot()
		progResp := &ProgressResponse{
			Stage:          snapshot.Stage,
			Percent:        snapshot.Percent,
			BytesProcessed: snapshot.BytesProcessed,
			BytesTotal:     snapshot.BytesTotal,
			Message:        snapshot.Message,
		}

		if snapshot.ETA != nil {
			etaStr := snapshot.ETA.Format(time.RFC3339)
			progResp.ETA = &etaStr
		}

		resp.Progress = progResp
	}

	return resp
}

// LogEntryToResponse converts a jobs.LogEntry to an API LogEntry
func LogEntryToResponse(entry jobs.LogEntry) LogEntry {
	return LogEntry{
		Timestamp: entry.Timestamp.Format(time.RFC3339),
		Level:     entry.Level,
		Message:   entry.Message,
	}
}
