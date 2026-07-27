package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/app"
	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/jobs"
)

// fakeJobStore is a jobs.JobStore backed by a fixed slice, so dashboard tests
// exercise the real merge/sort path in jobs.Manager instead of reaching into
// its unexported map.
type fakeJobStore struct {
	history []*jobs.Job
}

func (f *fakeJobStore) CreateJob(context.Context, *jobs.Job) error { return nil }
func (f *fakeJobStore) UpdateJob(context.Context, *jobs.Job) error { return nil }

func (f *fakeJobStore) GetJob(_ context.Context, id jobs.JobID) (*jobs.Job, error) {
	for _, job := range f.history {
		if job.ID == id {
			return job, nil
		}
	}
	return nil, fmt.Errorf("job not found: %s", id)
}

func (f *fakeJobStore) ListJobs(_ context.Context, filter jobs.JobFilter) ([]*jobs.Job, error) {
	matched := make([]*jobs.Job, 0, len(f.history))
	for _, job := range f.history {
		if filter.TargetName != "" && job.TargetName != filter.TargetName {
			continue
		}
		if filter.Status != "" && job.GetStatus() != filter.Status {
			continue
		}
		if filter.Type != "" && job.Type != filter.Type {
			continue
		}
		matched = append(matched, job)
	}
	return matched, nil
}

func (f *fakeJobStore) SaveJobLogsBatch(context.Context, jobs.JobID, []jobs.LogEntry) error {
	return nil
}

func (f *fakeJobStore) GetJobLogs(context.Context, jobs.JobID, int) ([]jobs.LogEntry, error) {
	return nil, nil
}

func (f *fakeJobStore) AcquireLock(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeJobStore) ReleaseLock(context.Context, string) error { return nil }
func (f *fakeJobStore) Close() error                              { return nil }

// backupJob builds a finished backup job. size <= 0 leaves the job without a
// result, mimicking history written before results were persisted.
func backupJob(target string, status jobs.JobStatus, finishedAt time.Time, size int64) *jobs.Job {
	started := finishedAt.Add(-30 * time.Second)
	created := started.Add(-time.Second)

	job := &jobs.Job{
		ID:          jobs.JobID(fmt.Sprintf("%s-%d", target, finishedAt.UnixNano())),
		Type:        jobs.JobTypeBackup,
		TargetName:  target,
		Status:      status,
		CreatedAt:   created,
		StartedAt:   &started,
		CompletedAt: &finishedAt,
	}

	if status == jobs.JobStatusCompleted && size > 0 {
		job.Result = &app.BackupResult{Target: target, Success: true, Size: size}
	}
	if status == jobs.JobStatusFailed {
		job.Error = "backup failed"
	}

	return job
}

func dashboardTarget(name, schedule string) *config.Target {
	return &config.Target{
		Name: name,
		Conn: &config.Connection{
			Type:     "mysql",
			Host:     "localhost",
			Port:     3306,
			Database: "appdb",
		},
		Schedule: schedule,
	}
}

// dashboardServer wires a Server around a fixed set of targets and history.
// store == nil models a daemon running without job persistence.
func dashboardServer(targets []*config.Target, history []*jobs.Job, persistent bool) *Server {
	cfg := &config.Config{Targets: targets}

	var store jobs.JobStore
	if persistent {
		store = &fakeJobStore{history: history}
	}

	return &Server{
		cfg:        cfg,
		jobManager: jobs.NewManager(cfg, store, nil, 2, 10),
	}
}

func requestDashboard(t *testing.T, server *Server) DashboardResponse {
	t.Helper()

	rr := httptest.NewRecorder()
	server.handleDashboard(rr, httptest.NewRequest("GET", "/api/dashboard", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var resp DashboardResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

func TestHandleDashboard_TargetHealth(t *testing.T) {
	now := time.Now()

	// "*/5 * * * *" fires every five minutes, so any reference point an hour
	// or more in the past is unambiguously overdue regardless of wall clock.
	const frequent = "*/5 * * * *"
	// "0 2 * * *" fires daily; measured from a success taken just now, the
	// next run is always still ahead.
	const daily = "0 2 * * *"

	tests := []struct {
		name    string
		targets []*config.Target
		history []*jobs.Job
		check   func(t *testing.T, resp DashboardResponse)
	}{
		{
			name:    "no jobs at all",
			targets: []*config.Target{dashboardTarget("fresh", daily)},
			history: nil,
			check: func(t *testing.T, resp DashboardResponse) {
				summary := resp.Targets[0]
				assert.Equal(t, backupOutcomeNever, summary.LastBackupStatus)
				assert.Equal(t, 0, summary.ConsecutiveFailures)
				assert.Nil(t, summary.LastBackup)
				assert.Nil(t, summary.LastBackupBytes)
				assert.Nil(t, summary.LastBackupDurationSeconds)
				// No history means no evidence the target ever had a run due.
				assert.False(t, summary.Overdue)
				assert.NotNil(t, summary.NextScheduled)

				assert.Nil(t, resp.SuccessRate24h, "no finished job is not a 0% success rate")
				require.NotNil(t, resp.FailedJobs24h)
				assert.Equal(t, 0, *resp.FailedJobs24h)
			},
		},
		{
			name:    "only failed jobs",
			targets: []*config.Target{dashboardTarget("broken", frequent)},
			history: []*jobs.Job{
				backupJob("broken", jobs.JobStatusFailed, now.Add(-1*time.Hour), 0),
				backupJob("broken", jobs.JobStatusFailed, now.Add(-2*time.Hour), 0),
				backupJob("broken", jobs.JobStatusFailed, now.Add(-3*time.Hour), 0),
			},
			check: func(t *testing.T, resp DashboardResponse) {
				summary := resp.Targets[0]
				assert.Equal(t, backupOutcomeFailed, summary.LastBackupStatus)
				assert.Equal(t, 3, summary.ConsecutiveFailures)
				assert.Nil(t, summary.LastBackup, "a failure is not a last backup")
				assert.Nil(t, summary.LastBackupBytes)
				assert.True(t, summary.Overdue)

				require.NotNil(t, resp.SuccessRate24h)
				assert.Equal(t, 0.0, *resp.SuccessRate24h)
				require.NotNil(t, resp.FailedJobs24h)
				assert.Equal(t, 3, *resp.FailedJobs24h)
			},
		},
		{
			name:    "mixed success and failure",
			targets: []*config.Target{dashboardTarget("flaky", daily)},
			history: []*jobs.Job{
				backupJob("flaky", jobs.JobStatusFailed, now, 0),
				backupJob("flaky", jobs.JobStatusCompleted, now.Add(-2*time.Hour), 4096),
				backupJob("flaky", jobs.JobStatusFailed, now.Add(-6*time.Hour), 0),
				// Cancelled jobs are neither a success nor a failure.
				backupJob("flaky", jobs.JobStatusCancelled, now.Add(-8*time.Hour), 0),
			},
			check: func(t *testing.T, resp DashboardResponse) {
				summary := resp.Targets[0]
				assert.Equal(t, backupOutcomeFailed, summary.LastBackupStatus,
					"the newest finished job failed")
				assert.Equal(t, 1, summary.ConsecutiveFailures,
					"only failures newer than the last success count")

				require.NotNil(t, summary.LastBackup, "a later failure does not erase the last good backup")
				assert.Equal(t, now.Add(-2*time.Hour).Format(time.RFC3339), *summary.LastBackup)
				require.NotNil(t, summary.LastBackupBytes)
				assert.Equal(t, int64(4096), *summary.LastBackupBytes)
				require.NotNil(t, summary.LastBackupDurationSeconds)
				assert.InDelta(t, 30.0, *summary.LastBackupDurationSeconds, 0.001)

				require.NotNil(t, resp.SuccessRate24h)
				assert.InDelta(t, 33.3, *resp.SuccessRate24h, 0.001)
				require.NotNil(t, resp.FailedJobs24h)
				assert.Equal(t, 2, *resp.FailedJobs24h)
			},
		},
		{
			name:    "schedule elapsed since last success",
			targets: []*config.Target{dashboardTarget("stale", frequent)},
			history: []*jobs.Job{
				backupJob("stale", jobs.JobStatusCompleted, now.Add(-72*time.Hour), 2048),
			},
			check: func(t *testing.T, resp DashboardResponse) {
				summary := resp.Targets[0]
				assert.Equal(t, backupOutcomeSuccess, summary.LastBackupStatus)
				assert.Equal(t, 0, summary.ConsecutiveFailures)
				require.NotNil(t, summary.LastBackup)
				assert.True(t, summary.Overdue)

				assert.Nil(t, resp.SuccessRate24h, "the only job is older than 24h")
				require.NotNil(t, resp.SuccessRate7d)
				assert.Equal(t, 100.0, *resp.SuccessRate7d)
			},
		},
		{
			name:    "target with no schedule",
			targets: []*config.Target{dashboardTarget("manual-only", "")},
			history: []*jobs.Job{
				backupJob("manual-only", jobs.JobStatusCompleted, now.Add(-30*24*time.Hour), 512),
			},
			check: func(t *testing.T, resp DashboardResponse) {
				summary := resp.Targets[0]
				assert.Empty(t, summary.Schedule)
				assert.Nil(t, summary.NextScheduled)
				assert.False(t, summary.Overdue, "an unscheduled target can never be late")
				assert.Equal(t, backupOutcomeSuccess, summary.LastBackupStatus)
				require.NotNil(t, summary.LastBackupBytes)
				assert.Equal(t, int64(512), *summary.LastBackupBytes)

				assert.Nil(t, resp.SuccessRate24h)
				assert.Nil(t, resp.SuccessRate7d, "the only job predates both windows")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := dashboardServer(tt.targets, tt.history, true)
			resp := requestDashboard(t, server)

			require.Len(t, resp.Targets, len(tt.targets))
			tt.check(t, resp)
		})
	}
}

func TestHandleDashboard_SevenDayRateNeedsPersistence(t *testing.T) {
	now := time.Now()
	targets := []*config.Target{dashboardTarget("t1", "0 2 * * *")}
	history := []*jobs.Job{backupJob("t1", jobs.JobStatusCompleted, now.Add(-time.Hour), 100)}

	withStore := requestDashboard(t, dashboardServer(targets, history, true))
	require.NotNil(t, withStore.SuccessRate7d)
	assert.Equal(t, 100.0, *withStore.SuccessRate7d)

	// In-memory history is pruned well inside seven days, so the window is
	// unanswerable rather than 100%.
	withoutStore := requestDashboard(t, dashboardServer(targets, nil, false))
	assert.Nil(t, withoutStore.SuccessRate7d)
}

func TestHandleDashboard_TotalStorageIsAbsent(t *testing.T) {
	now := time.Now()
	server := dashboardServer(
		[]*config.Target{dashboardTarget("t1", "")},
		[]*jobs.Job{backupJob("t1", jobs.JobStatusCompleted, now, 8192)},
		true,
	)

	rr := httptest.NewRecorder()
	server.handleDashboard(rr, httptest.NewRequest("GET", "/api/dashboard", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &raw))

	// Nothing tracks bytes currently held in storage, so the key must be
	// missing rather than present and zero.
	_, present := raw["total_storage_bytes"]
	assert.False(t, present, "total_storage_bytes must be omitted, not reported as 0")
}

// TestHandleListTargets_SharesTargetSummary pins the contract that both
// endpoints render a target the same way.
func TestHandleListTargets_SharesTargetSummary(t *testing.T) {
	now := time.Now()
	targets := []*config.Target{dashboardTarget("shared", "0 2 * * *")}
	history := []*jobs.Job{backupJob("shared", jobs.JobStatusCompleted, now, 1024)}

	server := dashboardServer(targets, history, true)

	rr := httptest.NewRecorder()
	server.handleListTargets(rr, httptest.NewRequest("GET", "/api/targets", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var listed ListTargetsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&listed))
	require.Len(t, listed.Targets, 1)

	dashboard := requestDashboard(t, server)
	require.Len(t, dashboard.Targets, 1)

	assert.Equal(t, dashboard.Targets[0].LastBackup, listed.Targets[0].LastBackup)
	assert.Equal(t, dashboard.Targets[0].LastBackupStatus, listed.Targets[0].LastBackupStatus)
	assert.Equal(t, dashboard.Targets[0].LastBackupBytes, listed.Targets[0].LastBackupBytes)
	assert.NotNil(t, listed.Targets[0].NextScheduled)
}

func TestIsOverdue(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	twoDaysAgo := now.Add(-48 * time.Hour)
	tenMinutesAgo := now.Add(-10 * time.Minute)

	tests := []struct {
		name     string
		schedule string
		health   *targetHealth
		want     bool
	}{
		{
			name:     "no schedule is never overdue",
			schedule: "",
			health:   &targetHealth{lastSuccessAt: &twoDaysAgo},
			want:     false,
		},
		{
			name:     "no history at all is not overdue",
			schedule: "0 2 * * *",
			health:   &targetHealth{outcome: backupOutcomeNever},
			want:     false,
		},
		{
			name:     "daily schedule, success two days ago",
			schedule: "0 2 * * *",
			health:   &targetHealth{lastSuccessAt: &twoDaysAgo},
			want:     true,
		},
		{
			name:     "daily schedule, success ten minutes ago",
			schedule: "0 2 * * *",
			health:   &targetHealth{lastSuccessAt: &tenMinutesAgo},
			want:     false,
		},
		{
			name:     "never succeeded, measured from oldest known job",
			schedule: "0 2 * * *",
			health:   &targetHealth{outcome: backupOutcomeFailed, oldestJobAt: &twoDaysAgo},
			want:     true,
		},
		{
			name:     "unparseable schedule is not overdue",
			schedule: "not a cron expression",
			health:   &targetHealth{lastSuccessAt: &twoDaysAgo},
			want:     false,
		},
		{
			name:     "nil health is not overdue",
			schedule: "0 2 * * *",
			health:   nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isOverdue(tt.schedule, tt.health, now))
		})
	}
}

func TestBackupWindowStats(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	restoreJob := func(finishedAt time.Time) *jobs.Job {
		job := backupJob("t1", jobs.JobStatusCompleted, finishedAt, 1)
		job.Type = jobs.JobTypeRestore
		return job
	}

	tests := []struct {
		name       string
		history    []*jobs.Job
		truncated  bool
		wantRate   *float64
		wantFailed *int
	}{
		{
			name:       "empty history has no rate but a real zero failure count",
			history:    nil,
			wantRate:   nil,
			wantFailed: ptr(0),
		},
		{
			name: "all successful",
			history: []*jobs.Job{
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-time.Hour), 1),
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-2*time.Hour), 1),
			},
			wantRate:   ptr(100.0),
			wantFailed: ptr(0),
		},
		{
			name: "two of three succeeded rounds to one decimal",
			history: []*jobs.Job{
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-time.Hour), 1),
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-2*time.Hour), 1),
				backupJob("t1", jobs.JobStatusFailed, now.Add(-3*time.Hour), 0),
			},
			wantRate:   ptr(66.7),
			wantFailed: ptr(1),
		},
		{
			name: "jobs outside the window are ignored",
			history: []*jobs.Job{
				backupJob("t1", jobs.JobStatusFailed, now.Add(-30*time.Hour), 0),
			},
			wantRate:   nil,
			wantFailed: ptr(0),
		},
		{
			name: "restore jobs do not count towards backup health",
			history: []*jobs.Job{
				restoreJob(now.Add(-time.Hour)),
			},
			wantRate:   nil,
			wantFailed: ptr(0),
		},
		{
			name: "truncated history that does not reach the window start is unanswerable",
			history: []*jobs.Job{
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-time.Hour), 1),
			},
			truncated:  true,
			wantRate:   nil,
			wantFailed: nil,
		},
		{
			name: "truncated history that reaches past the window start still answers",
			history: []*jobs.Job{
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-time.Hour), 1),
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-40*time.Hour), 1),
			},
			truncated:  true,
			wantRate:   ptr(100.0),
			wantFailed: ptr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, failed := backupWindowStats(tt.history, tt.truncated, window24h, now)

			if tt.wantRate == nil {
				assert.Nil(t, rate)
			} else {
				require.NotNil(t, rate)
				assert.InDelta(t, *tt.wantRate, *rate, 0.001)
			}

			if tt.wantFailed == nil {
				assert.Nil(t, failed)
			} else {
				require.NotNil(t, failed)
				assert.Equal(t, *tt.wantFailed, *failed)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
