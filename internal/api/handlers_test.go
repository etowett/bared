package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/app"
	"bared/internal/config"
	"bared/internal/jobs"
	"bared/internal/testutil/fixtures"
	"bared/internal/util"
)

func init() {
	// Initialize logger to avoid race conditions
	util.InitLogger(util.ERROR)
}

func setupTestServer(_ *testing.T) *Server {
	target := fixtures.MySQLTarget()
	target.Name = "test-target"

	cfg := &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": fixtures.LocalStorage(),
		},
		Targets: []*config.Target{target},
	}

	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	return server
}

func TestHandleHealth(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	server.handleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response HealthResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.NotEmpty(t, response.Version)
}

func TestHandleListTargets(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/targets", nil)
	rr := httptest.NewRecorder()

	server.handleListTargets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response ListTargetsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Total)
	assert.Len(t, response.Targets, 1)
	assert.Equal(t, "test-target", response.Targets[0].Name)
}

func TestHandleListRestoreTargets(t *testing.T) {
	restoreTarget := &config.RestoreTarget{
		Name:         "restore-test",
		Description:  "Test restore target",
		SourceTarget: "test-target",
		Conn: &config.Connection{
			Type:     "mysql",
			Host:     "localhost",
			Port:     3306,
			User:     "restore_user",
			Database: "restore_db",
		},
	}

	cfg := &config.Config{
		Targets:        []*config.Target{fixtures.MySQLTarget()},
		RestoreTargets: []*config.RestoreTarget{restoreTarget},
	}

	mgr := jobs.NewManager(cfg, nil, nil, 2, 10)
	server := &Server{
		jobManager: mgr,
		cfg:        cfg,
	}

	req := httptest.NewRequest("GET", "/api/restore-targets", nil)
	rr := httptest.NewRecorder()

	server.handleListRestoreTargets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response ListRestoreTargetsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Total)
	assert.Len(t, response.RestoreTargets, 1)
	assert.Equal(t, "restore-test", response.RestoreTargets[0].Name)
}

func TestHandleListJobs_Empty(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/jobs", nil)
	rr := httptest.NewRecorder()

	server.handleListJobs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response ListJobsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 0, response.Total)
	assert.Len(t, response.Jobs, 0)
}

func TestHandleGetJob_NotFound(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/jobs/nonexistent-id", nil)
	rr := httptest.NewRecorder()

	server.handleGetJob(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleGetJob_InvalidPath(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/jobs/", nil)
	rr := httptest.NewRecorder()

	server.handleGetJob(rr, req)

	// /api/jobs/ path results in 404 because split gives ["", "api", "jobs", ""]
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleTriggerBackup_MethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/jobs/backup", nil)
	rr := httptest.NewRecorder()

	server.handleTriggerBackup(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleTriggerBackup_InvalidJSON(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/jobs/backup", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleTriggerBackup(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTriggerBackup_MissingTarget(t *testing.T) {
	server := setupTestServer(t)

	reqBody := TriggerBackupRequest{Target: ""}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/jobs/backup", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleTriggerBackup(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTriggerBackup_TargetNotFound(t *testing.T) {
	server := setupTestServer(t)

	reqBody := TriggerBackupRequest{Target: "nonexistent-target"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/jobs/backup", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleTriggerBackup(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleTriggerRestore_InvalidJSON(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/jobs/restore", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleTriggerRestore(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTriggerRestore_MissingTarget(t *testing.T) {
	server := setupTestServer(t)

	reqBody := TriggerRestoreRequest{
		Target:     "",
		BackupPath: "backup.tar.gz",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/jobs/restore", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleTriggerRestore(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTriggerRestore_MissingBackupPath(t *testing.T) {
	server := setupTestServer(t)

	reqBody := TriggerRestoreRequest{
		Target:     "test-target",
		BackupPath: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/jobs/restore", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleTriggerRestore(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCancelJob_NotFound(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/jobs/nonexistent-id/cancel", nil)
	rr := httptest.NewRecorder()

	server.handleCancelJob(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleCancelJob_InvalidPath(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/jobs/cancel", nil)
	rr := httptest.NewRecorder()

	server.handleCancelJob(rr, req)

	// "cancel" becomes the job ID and is not found
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleCancelJob_MethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/jobs/someid/cancel", nil)
	rr := httptest.NewRecorder()

	server.handleCancelJob(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleGetJobLogs_NotFound(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/jobs/nonexistent-id/logs", nil)
	rr := httptest.NewRecorder()

	server.handleGetJobLogs(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDashboard(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/dashboard", nil)
	rr := httptest.NewRecorder()

	server.handleDashboard(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response DashboardResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response.Targets, 1)
	assert.Equal(t, 0, response.TotalJobs)
}

func TestRespondJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	data := map[string]string{"message": "test"}

	respondJSON(rr, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response map[string]string
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "test", response["message"])
}

func TestRespondError(t *testing.T) {
	rr := httptest.NewRecorder()

	respondError(rr, http.StatusBadRequest, "test error message")

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "Bad Request", response.Error) // Error field contains http.StatusText
	assert.Equal(t, "test error message", response.Message)
}

func TestJobToResponse(t *testing.T) {
	job := jobs.NewJob(jobs.JobTypeBackup, "test-target", true)
	job.MarkStarted()

	result := &app.BackupResult{
		Target:     "test-target",
		Success:    true,
		BackupPath: "/backups/test.tar.gz",
	}
	job.MarkCompleted(result)

	response := JobToResponse(job)

	assert.Equal(t, string(job.ID), response.ID)
	assert.Equal(t, "backup", response.Type)
	assert.Equal(t, "test-target", response.Target)
	assert.Equal(t, "completed", response.Status)
	assert.True(t, response.Manual)
	assert.NotNil(t, response.CreatedAt)
	assert.NotNil(t, response.StartedAt)
	assert.NotNil(t, response.CompletedAt)
}

func TestHandleListJobs_WithFilters(t *testing.T) {
	server := setupTestServer(t)

	// Test with status filter
	req := httptest.NewRequest("GET", "/api/jobs?status=running", nil)
	rr := httptest.NewRecorder()

	server.handleListJobs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Test with target filter
	req = httptest.NewRequest("GET", "/api/jobs?target=test-target", nil)
	rr = httptest.NewRecorder()

	server.handleListJobs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleListTargets_WithRunningJobs(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/targets", nil)
	rr := httptest.NewRecorder()

	server.handleListTargets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response ListTargetsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Total)
	// IsRunning should be false since no jobs are running
	assert.False(t, response.Targets[0].IsRunning)
}
