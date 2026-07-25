// Package api provides HTTP API handlers for the BareD REST API.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/robfig/cron/v3"

	"bared/internal/app"
	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/util"
	"bared/internal/version"
)

// calculateNextRun calculates the next execution time for a cron expression
func calculateNextRun(cronExpr string) (*time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return nil, err
	}

	next := schedule.Next(time.Now())
	return &next, nil
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

	// Try to get targets from configService first (database), fall back to static config
	var targets []*config.Target
	if s.configService != nil {
		dbTargets, err := s.configService.ListTargets(ctx)
		if err == nil {
			targets = dbTargets
		} else {
			// Fall back to static config if database read fails
			targets = s.cfg.Targets
		}
	} else {
		// No configService, use static config
		targets = s.cfg.Targets
	}

	summaries := make([]TargetSummary, 0, len(targets))

	for _, target := range targets {
		summary := TargetSummary{
			Name:      target.Name,
			Type:      target.Conn.Type,
			Database:  target.Conn.Database,
			Schedule:  target.Schedule,
			IsRunning: s.jobManager.IsTargetRunning(target.Name),
		}

		// Calculate next scheduled run if target has a schedule
		if target.Schedule != "" {
			nextRun, err := calculateNextRun(target.Schedule)
			if err == nil && nextRun != nil {
				nextRunStr := nextRun.Format(time.RFC3339)
				summary.NextScheduled = &nextRunStr
			}
		}

		// TODO: Add last backup time from storage

		summaries = append(summaries, summary)
	}

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

	// Try to get targets from configService first (database), fall back to static config
	var targets []*config.Target
	if s.configService != nil {
		dbTargets, err := s.configService.ListTargets(ctx)
		if err == nil {
			targets = dbTargets
		} else {
			// Fall back to static config if database read fails
			targets = s.cfg.Targets
		}
	} else {
		// No configService, use static config
		targets = s.cfg.Targets
	}

	summaries := make([]TargetSummary, 0, len(targets))

	for _, target := range targets {
		summary := TargetSummary{
			Name:      target.Name,
			Type:      target.Conn.Type,
			Database:  target.Conn.Database,
			Schedule:  target.Schedule,
			IsRunning: s.jobManager.IsTargetRunning(target.Name),
		}
		summaries = append(summaries, summary)
	}

	// Get job counts
	allJobs := s.jobManager.ListJobs(r.Context())
	activeJobs := 0
	for _, job := range allJobs {
		status := job.GetStatus()
		if status == jobs.JobStatusRunning || status == jobs.JobStatusQueued {
			activeJobs++
		}
	}

	respondJSON(w, http.StatusOK, DashboardResponse{
		Targets:    summaries,
		ActiveJobs: activeJobs,
		TotalJobs:  len(allJobs),
	})
}
