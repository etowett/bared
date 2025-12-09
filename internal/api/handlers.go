// Package api provides HTTP API handlers for the BareD REST API.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"bared/internal/app"
	"bared/internal/jobs"
	"bared/internal/version"
)

// handleHealth returns the API health status
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: version.GetVersion(),
	})
}

// handleListTargets returns all configured targets
func (s *Server) handleListTargets(w http.ResponseWriter, _ *http.Request) {
	targets := s.cfg.Targets
	summaries := make([]TargetSummary, 0, len(targets))

	for _, target := range targets {
		summary := TargetSummary{
			Name:      target.Name,
			Type:      target.Conn.Type,
			Database:  target.Conn.Database,
			Schedule:  target.Schedule,
			IsRunning: s.jobManager.IsTargetRunning(target.Name),
		}

		// TODO: Add last backup time from storage
		// TODO: Add next scheduled time from cron

		summaries = append(summaries, summary)
	}

	respondJSON(w, http.StatusOK, ListTargetsResponse{
		Targets: summaries,
		Total:   len(summaries),
	})
}

// handleListRestoreTargets returns all configured restore targets
func (s *Server) handleListRestoreTargets(w http.ResponseWriter, _ *http.Request) {
	restoreTargets := s.cfg.RestoreTargets
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
	status := r.URL.Query().Get("status")
	target := r.URL.Query().Get("target")

	// Get all jobs
	allJobs := s.jobManager.ListJobs()

	// Filter jobs
	var filteredJobs []*jobs.Job
	for _, job := range allJobs {
		// Filter by status if specified
		if status != "" && string(job.GetStatus()) != status {
			continue
		}

		// Filter by target if specified
		if target != "" && job.TargetName != target {
			continue
		}

		filteredJobs = append(filteredJobs, job)
	}

	// Convert to response format
	responses := make([]JobResponse, 0, len(filteredJobs))
	for _, job := range filteredJobs {
		responses = append(responses, JobToResponse(job))
	}

	respondJSON(w, http.StatusOK, ListJobsResponse{
		Jobs:  responses,
		Total: len(responses),
	})
}

// handleGetJob returns a specific job
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}
	jobIDStr := parts[3] // /api/jobs/{id}

	jobID := jobs.JobID(jobIDStr)

	job, err := s.jobManager.GetJob(jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, JobToResponse(job))
}

// handleTriggerBackup triggers a manual backup
func (s *Server) handleTriggerBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse request body
	var req TriggerBackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Target == "" {
		respondError(w, http.StatusBadRequest, "Target name is required")
		return
	}

	// Find target
	target, err := s.cfg.FindTarget(req.Target)
	if err != nil {
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
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

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

	// Resolve target (regular or restore target)
	target, _, _, err := s.cfg.ResolveRestoreTarget(req.Target)
	if err != nil {
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
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract job ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}
	jobIDStr := parts[3] // /api/jobs/{id}

	jobID := jobs.JobID(jobIDStr)

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
	// Extract job ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}
	jobIDStr := parts[3] // /api/jobs/{id}/logs

	jobID := jobs.JobID(jobIDStr)

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
func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	// Get all targets
	targets := s.cfg.Targets
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
	allJobs := s.jobManager.ListJobs()
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
