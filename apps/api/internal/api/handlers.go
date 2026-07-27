// Package api provides HTTP API handlers for the BareD REST API.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/robfig/cron/v3"

	"github.com/etowett/bared/apps/api/internal/app"
	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/jobs"
	"github.com/etowett/bared/apps/api/internal/util"
	"github.com/etowett/bared/apps/api/internal/version"
)

// parseCronSchedule parses a target's cron expression using the same dialect
// the daemon schedules with.
func parseCronSchedule(cronExpr string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return parser.Parse(cronExpr)
}

// calculateNextRun calculates the next execution time for a cron expression
func calculateNextRun(cronExpr string) (*time.Time, error) {
	schedule, err := parseCronSchedule(cronExpr)
	if err != nil {
		return nil, err
	}

	next := schedule.Next(time.Now())
	return &next, nil
}

// nextScheduledRun renders a target's next run as RFC3339, or nil when the
// target has no schedule or its expression does not parse. Shared by every
// handler that reports a TargetSummary so the two never drift apart.
func nextScheduledRun(cronExpr string) *string {
	if cronExpr == "" {
		return nil
	}

	next, err := calculateNextRun(cronExpr)
	if err != nil || next == nil {
		return nil
	}

	formatted := next.Format(time.RFC3339)
	return &formatted
}

// resolveTargets reads targets from the config service (database) and falls
// back to the static config when the database is unavailable.
func (s *Server) resolveTargets(ctx context.Context) []*config.Target {
	if s.configService != nil {
		dbTargets, err := s.configService.ListTargets(ctx)
		if err == nil {
			return dbTargets
		}
		// The fallback feeds the health view, so a silent swap to a stale set
		// of targets would be invisible to the operator watching it.
		util.GetLogger().ErrorS("Failed to read targets from config store, falling back to static config",
			"component", "api",
			"error", err)
	}
	return s.cfg.Targets
}

// buildTargetSummaries renders targets with their scheduling and health rollup.
// Both /api/targets and /api/dashboard go through here so the two endpoints
// cannot disagree about a target's state.
//
// A target missing from the sample reports "never" only when the sample is
// complete. Otherwise it reports "unknown": a target whose history was cut off
// by the row cap, or lost with the job store, is not a brand-new target, and
// rendering it as one turns an alarming state into a reassuring one.
func (s *Server) buildTargetSummaries(targets []*config.Target, history historySample, now time.Time) []TargetSummary {
	health := computeTargetHealth(history.jobs)
	summaries := make([]TargetSummary, 0, len(targets))

	for _, target := range targets {
		entry := health[target.Name]
		running := s.jobManager.IsTargetRunning(target.Name)

		summary := TargetSummary{
			Name:             target.Name,
			Type:             target.Conn.Type,
			Database:         target.Conn.Database,
			Schedule:         target.Schedule,
			IsRunning:        running,
			NextScheduled:    nextScheduledRun(target.Schedule),
			LastBackupStatus: backupOutcomeNever,
		}

		switch {
		case entry != nil:
			summary.LastBackupStatus = entry.outcome
			summary.ConsecutiveFailures = entry.consecutiveFailures
			summary.LastBackupBytes = entry.lastSuccessBytes
			summary.LastBackupDurationSeconds = entry.lastSuccessSeconds

			if entry.lastSuccessAt != nil {
				lastBackup := entry.lastSuccessAt.Format(time.RFC3339)
				summary.LastBackup = &lastBackup
			}

			// With the store unreachable, a missing last success means "not
			// since this process started", not "not for two days" — measuring
			// lateness from it would invent an alarm.
			summary.Overdue = !history.degraded && isOverdue(target.Schedule, entry, running, now)

		case !history.complete():
			summary.LastBackupStatus = backupOutcomeUnknown
		}

		summaries = append(summaries, summary)
	}

	return summaries
}

// backupHistory fetches the backup jobs the rollups are derived from, newest
// first, along with what the fetch could not cover.
func (s *Server) backupHistory(ctx context.Context) historySample {
	history, err := s.jobManager.ListJobsFilteredE(ctx, jobs.JobFilter{
		Type:  jobs.JobTypeBackup,
		Limit: dashboardHistoryLimit,
		// The rollups report the artifact size of the last successful backup,
		// which only exists in the persisted result.
		WithResults: true,
	})
	if err != nil {
		util.GetLogger().ErrorS("Backup history is incomplete: job store read failed",
			"component", "api",
			"error", err)
		return historySample{jobs: history, degraded: true}
	}

	return historySample{jobs: history, truncated: len(history) >= dashboardHistoryLimit}
}

// jobToResponse converts a job to a response with schedule information
func (s *Server) jobToResponse(job *jobs.Job) JobResponse {
	resp := JobToResponse(job)

	// Add schedule context if target exists in config
	if s.cfg != nil {
		for _, target := range s.cfg.Targets {
			if target.Name == job.TargetName {
				// Add target's schedule if available
				if target.Schedule != "" {
					resp.TargetSchedule = &target.Schedule
				}

				// Determine trigger type
				triggeredBy := "manual"
				if !job.Manual {
					triggeredBy = "schedule"
				}
				resp.TriggeredBy = &triggeredBy
				break
			}
		}
	}

	return resp
}

// handleHealth returns the API health status
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: version.GetVersion(),
	})
}

// handleListTargets returns all configured targets
func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	targets := s.resolveTargets(ctx)
	summaries := s.buildTargetSummaries(targets, s.backupHistory(ctx), time.Now())

	respondJSON(w, http.StatusOK, ListTargetsResponse{
		Targets: summaries,
		Total:   len(summaries),
	})
}

// handleListRestoreTargets returns all configured restore targets
func (s *Server) handleListRestoreTargets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to get restore targets from configService first (database), fall back to static config
	var restoreTargets []*config.RestoreTarget
	if s.configService != nil {
		dbRestoreTargets, err := s.configService.ListRestoreTargets(ctx)
		if err == nil {
			restoreTargets = dbRestoreTargets
		} else {
			// Fall back to static config if database read fails
			restoreTargets = s.cfg.RestoreTargets
		}
	} else {
		// No configService, use static config
		restoreTargets = s.cfg.RestoreTargets
	}

	summaries := make([]RestoreTargetSummary, 0, len(restoreTargets))

	for _, rt := range restoreTargets {
		summary := RestoreTargetSummary{
			Name:         rt.Name,
			Type:         rt.Conn.Type,
			Database:     rt.Conn.Database,
			Host:         rt.Conn.Host,
			Description:  rt.Description,
			SourceTarget: rt.SourceTarget,
		}
		summaries = append(summaries, summary)
	}

	respondJSON(w, http.StatusOK, ListRestoreTargetsResponse{
		RestoreTargets: summaries,
		Total:          len(summaries),
	})
}

// handleListJobs returns all jobs or filtered jobs
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	statusStr := r.URL.Query().Get("status")
	target := r.URL.Query().Get("target")
	typeStr := r.URL.Query().Get("type")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Parse pagination parameters with defaults
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Parse filters (keep backward-compatible semantics: invalid filter => empty result)
	var status jobs.JobStatus
	if statusStr != "" {
		switch statusStr {
		case string(jobs.JobStatusQueued),
			string(jobs.JobStatusRunning),
			string(jobs.JobStatusCompleted),
			string(jobs.JobStatusFailed),
			string(jobs.JobStatusCancelling),
			string(jobs.JobStatusCancelled):
			status = jobs.JobStatus(statusStr)
		default:
			respondJSON(w, http.StatusOK, ListJobsResponse{
				Jobs:  []JobResponse{},
				Total: 0,
				Pagination: PaginationMetadata{
					Page:       page,
					Limit:      limit,
					Offset:     0,
					TotalPages: 0,
					HasNext:    false,
					HasPrev:    false,
				},
			})
			return
		}
	}

	var jobType jobs.JobType
	if typeStr != "" {
		switch typeStr {
		case string(jobs.JobTypeBackup), string(jobs.JobTypeRestore):
			jobType = jobs.JobType(typeStr)
		default:
			respondJSON(w, http.StatusOK, ListJobsResponse{
				Jobs:  []JobResponse{},
				Total: 0,
				Pagination: PaginationMetadata{
					Page:       page,
					Limit:      limit,
					Offset:     0,
					TotalPages: 0,
					HasNext:    false,
					HasPrev:    false,
				},
			})
			return
		}
	}

	filteredJobs := s.jobManager.ListJobsFiltered(r.Context(), jobs.JobFilter{
		TargetName: target,
		Status:     status,
		Type:       jobType,
	})

	total := len(filteredJobs)

	// Calculate pagination
	offset := (page - 1) * limit
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// Apply pagination to filtered jobs
	var paginatedJobs []*jobs.Job
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		paginatedJobs = filteredJobs[offset:end]
	} else {
		paginatedJobs = []*jobs.Job{}
	}

	// Convert to response format
	responses := make([]JobResponse, 0, len(paginatedJobs))
	for _, job := range paginatedJobs {
		responses = append(responses, s.jobToResponse(job))
	}

	// Calculate pagination metadata
	pagination := PaginationMetadata{
		Page:       page,
		Limit:      limit,
		Offset:     offset,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	respondJSON(w, http.StatusOK, ListJobsResponse{
		Jobs:       responses,
		Total:      total,
		Pagination: pagination,
	})
}

// handleGetJob returns a specific job
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := jobs.JobID(chi.URLParam(r, "id"))

	job, err := s.jobManager.GetJob(jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, s.jobToResponse(job))
}

// handleTriggerBackup triggers a manual backup
func (s *Server) handleTriggerBackup(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req TriggerBackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger := util.GetLogger()
	logger.InfoS("Triggering backup",
		"component", "api",
		"target", req.Target)

	if req.Target == "" {
		respondError(w, http.StatusBadRequest, "Target name is required")
		return
	}

	// Find target - try database first, fall back to static config
	var target *config.Target
	var err error
	if s.configService != nil {
		target, err = s.configService.GetTarget(r.Context(), req.Target)
		if err != nil {
			// Fall back to static config if database read fails
			target, err = s.cfg.FindTarget(req.Target)
		}
	} else {
		// No configService, use static config
		target, err = s.cfg.FindTarget(req.Target)
	}

	if err != nil || target == nil {
		respondError(w, http.StatusNotFound, "Target not found")
		return
	}

	// Submit backup job
	jobID, err := s.jobManager.SubmitBackup(r.Context(), target, true)
	if err != nil {
		if strings.Contains(err.Error(), "already in progress") {
			respondError(w, http.StatusConflict, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusAccepted, JobCreatedResponse{
		JobID:   string(jobID),
		Message: "Backup job queued successfully",
	})
}

// handleTriggerRestore triggers a manual restore
func (s *Server) handleTriggerRestore(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req TriggerRestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Target == "" {
		respondError(w, http.StatusBadRequest, "Target name is required")
		return
	}

	if req.BackupPath == "" {
		respondError(w, http.StatusBadRequest, "Backup path is required")
		return
	}

	// Resolve target (regular or restore target) - try database first, fall back to static config
	var target *config.Target
	var err error
	if s.configService != nil {
		// Try restore target first
		restoreTarget, rtErr := s.configService.GetRestoreTarget(r.Context(), req.Target)
		if rtErr == nil {
			// Convert RestoreTarget to Target for restore operations
			target = &config.Target{
				Name:    restoreTarget.Name,
				Conn:    restoreTarget.Conn,
				Storage: restoreTarget.Storage,
			}
		} else {
			// Fall back to regular target
			target, err = s.configService.GetTarget(r.Context(), req.Target)
			if err != nil {
				// Fall back to static config if database read fails
				target, _, _, err = s.cfg.ResolveRestoreTarget(req.Target)
			}
		}
	} else {
		// No configService, use static config
		target, _, _, err = s.cfg.ResolveRestoreTarget(req.Target)
	}

	if err != nil || target == nil {
		respondError(w, http.StatusNotFound, "Target or restore target not found")
		return
	}

	// Submit restore job with dry-run flag
	options := &app.RestoreOptions{
		DryRun: req.DryRun,
	}
	jobID, err := s.jobManager.SubmitRestoreWithOptions(r.Context(), target, req.BackupPath, true, options)
	if err != nil {
		if strings.Contains(err.Error(), "already in progress") {
			respondError(w, http.StatusConflict, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	message := "Restore job queued successfully"
	if req.DryRun {
		message = "Restore validation job queued successfully (dry-run)"
	}

	respondJSON(w, http.StatusAccepted, JobCreatedResponse{
		JobID:   string(jobID),
		Message: message,
	})
}

// handleCancelJob cancels a running job
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := jobs.JobID(chi.URLParam(r, "id"))

	err := s.jobManager.CancelJob(jobID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondSuccess(w, "Job cancellation requested")
}

// handleGetJobLogs returns logs for a specific job
func (s *Server) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	jobID := jobs.JobID(chi.URLParam(r, "id"))

	job, err := s.jobManager.GetJob(jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Get all logs
	logs := job.Logs.GetAll()

	// Convert to API format
	apiLogs := make([]LogEntry, 0, len(logs))
	for _, entry := range logs {
		apiLogs = append(apiLogs, LogEntryToResponse(entry))
	}

	respondJSON(w, http.StatusOK, LogsResponse{
		JobID: string(jobID),
		Logs:  apiLogs,
		Total: len(apiLogs),
	})
}

// handleDashboard returns dashboard summary
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	targets := s.resolveTargets(ctx)
	history := s.backupHistory(ctx)
	summaries := s.buildTargetSummaries(targets, history, now)

	// Get job counts
	allJobs := s.jobManager.ListJobs(ctx)
	activeJobs := 0
	for _, job := range allJobs {
		status := job.GetStatus()
		if status == jobs.JobStatusRunning || status == jobs.JobStatusQueued {
			activeJobs++
		}
	}

	resp := DashboardResponse{
		Targets:    summaries,
		ActiveJobs: activeJobs,
		TotalJobs:  len(allJobs),
	}

	// Without a job store, history is in-memory only: it starts when the daemon
	// starts and is pruned long before 7 days elapse, so a rate over a window
	// the process has not been up for would be computed from whatever handful
	// of jobs it has seen. Omit any such window rather than publish a figure
	// that flatters or alarms for no reason.
	if s.jobManager.CoversWindow(window24h, now) {
		resp.SuccessRate24h, resp.FailedJobs24h = backupWindowStats(history, window24h, now)
	}
	if s.jobManager.CoversWindow(window7d, now) {
		resp.SuccessRate7d, _ = backupWindowStats(history, window7d, now)
	}

	respondJSON(w, http.StatusOK, resp)
}
