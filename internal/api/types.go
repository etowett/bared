package api

import (
	"time"

	"bared/internal/jobs"
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
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Target      string            `json:"target"`
	Status      string            `json:"status"`
	Progress    *ProgressResponse `json:"progress,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   string            `json:"created_at"`
	StartedAt   *string           `json:"started_at,omitempty"`
	CompletedAt *string           `json:"completed_at,omitempty"`
	Duration    *float64          `json:"duration_seconds,omitempty"`
	Manual      bool              `json:"manual"`
	BackupPath  string            `json:"backup_path,omitempty"`
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

// ListJobsResponse represents the response for listing jobs
type ListJobsResponse struct {
	Jobs  []JobResponse `json:"jobs"`
	Total int           `json:"total"`
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

// TargetSummary represents a target's status
type TargetSummary struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Database      string  `json:"database"`
	LastBackup    *string `json:"last_backup,omitempty"`
	NextScheduled *string `json:"next_scheduled,omitempty"`
	Schedule      string  `json:"schedule,omitempty"`
	IsRunning     bool    `json:"is_running"`
}

// ListTargetsResponse represents the response for listing targets
type ListTargetsResponse struct {
	Targets []TargetSummary `json:"targets"`
	Total   int             `json:"total"`
}

// DashboardResponse represents the dashboard summary
type DashboardResponse struct {
	Targets      []TargetSummary `json:"targets"`
	ActiveJobs   int             `json:"active_jobs"`
	TotalJobs    int             `json:"total_jobs"`
	TotalStorage int64           `json:"total_storage_bytes,omitempty"`
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
	if job.Error != nil {
		resp.Error = job.Error.Error()
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
