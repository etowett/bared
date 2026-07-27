package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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

	// listErr, when set, is what ListJobs returns — a daemon whose job
	// database is unreachable.
	listErr error
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

// ListJobs mirrors the SQL store: filter, order by created_at descending, then
// apply LIMIT/OFFSET. The ordering and the cap are what produce the truncated
// samples the dashboard has to stay honest about, so a fake that ignored them
// would never exercise that path.
func (f *fakeJobStore) ListJobs(_ context.Context, filter jobs.JobFilter) ([]*jobs.Job, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

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

	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(matched) {
		return []*jobs.Job{}, nil
	}
	matched = matched[offset:]

	limit := filter.Limit
	if limit <= 0 {
		limit = 1000 // the SQL store's default page size
	}
	if limit < len(matched) {
		matched = matched[:limit]
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

// backupJob builds a finished backup job that ran for 30 seconds. size <= 0
// leaves the job without a result, mimicking history written before results
// were persisted.
func backupJob(target string, status jobs.JobStatus, finishedAt time.Time, size int64) *jobs.Job {
	return backupJobAt(target, status, finishedAt.Add(-31*time.Second), finishedAt, size)
}

// backupJobAt builds a finished backup job with its creation and completion
// times set independently, which is how a long-running or overlapping backup
// ends up finishing out of creation order.
func backupJobAt(target string, status jobs.JobStatus, created, finishedAt time.Time, size int64) *jobs.Job {
	started := created.Add(time.Second)

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
// persistent == false models a daemon running without job persistence.
func dashboardServer(targets []*config.Target, history []*jobs.Job, persistent bool) *Server {
	if !persistent {
		return dashboardServerWithStore(targets, nil)
	}
	return dashboardServerWithStore(targets, &fakeJobStore{history: history})
}

// dashboardServerWithStore wires a Server around a caller-supplied store, for
// the cases where the store misbehaves rather than merely holding jobs. A nil
// store means no job persistence at all.
func dashboardServerWithStore(targets []*config.Target, store jobs.JobStore) *Server {
	cfg := &config.Config{Targets: targets}

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
			name:    "the job that finished last wins, not the one created last",
			targets: []*config.Target{dashboardTarget("overlapping", daily)},
			history: []*jobs.Job{
				// Created second, finished first: a short run that started
				// after a long one and beat it to the end.
				backupJobAt("overlapping", jobs.JobStatusFailed,
					now.Add(-2*time.Hour), now.Add(-90*time.Minute), 0),
				// Created first, finished last. History is ordered by
				// CreatedAt, so this job is second in the slice.
				backupJobAt("overlapping", jobs.JobStatusCompleted,
					now.Add(-3*time.Hour), now.Add(-time.Hour), 111),
			},
			check: func(t *testing.T, resp DashboardResponse) {
				summary := resp.Targets[0]
				assert.Equal(t, backupOutcomeSuccess, summary.LastBackupStatus,
					"the newest job by completion succeeded")
				require.NotNil(t, summary.LastBackup)
				assert.Equal(t, now.Add(-time.Hour).Format(time.RFC3339), *summary.LastBackup)
				require.NotNil(t, summary.LastBackupBytes)
				assert.Equal(t, int64(111), *summary.LastBackupBytes,
					"the size must come from the run that last_backup names")
				assert.Equal(t, 0, summary.ConsecutiveFailures,
					"the failure finished before the success, so it is not consecutive")
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

// TestHandleDashboard_StoreFailureIsUnknownNotNever pins the one answer the
// dashboard must never give: a persistence outage renders as "I cannot tell",
// not as a wall of healthy, never-backed-up targets.
func TestHandleDashboard_StoreFailureIsUnknownNotNever(t *testing.T) {
	targets := []*config.Target{
		dashboardTarget("prod", "0 2 * * *"),
		dashboardTarget("staging", "0 3 * * *"),
	}
	store := &fakeJobStore{listErr: errors.New("dial tcp 10.0.0.5:5432: connect: connection refused")}

	resp := requestDashboard(t, dashboardServerWithStore(targets, store))
	require.Len(t, resp.Targets, 2)

	for _, summary := range resp.Targets {
		assert.Equal(t, backupOutcomeUnknown, summary.LastBackupStatus, summary.Name)
		assert.NotEqual(t, backupOutcomeNever, summary.LastBackupStatus,
			"a target with unreadable history has not 'never' been backed up")
		assert.False(t, summary.Overdue, "lateness cannot be measured from history we could not read")
		assert.Nil(t, summary.LastBackup)
		assert.Equal(t, 0, summary.ConsecutiveFailures)
	}

	assert.Nil(t, resp.SuccessRate24h)
	assert.Nil(t, resp.SuccessRate7d)
	assert.Nil(t, resp.FailedJobs24h, "zero failures is a claim; we have no basis for it")

	// A queued or running job puts the target in the rollup with nothing
	// finished in it. That must not read as "never": the outcome is what is
	// unknown, not merely the target's presence in the sample.
	queued := &jobs.Job{
		ID:         "queued-1",
		Type:       jobs.JobTypeBackup,
		TargetName: "prod",
		Status:     jobs.JobStatusQueued,
		CreatedAt:  time.Now().Add(-time.Minute),
	}
	withQueued := dashboardServerWithStore(targets, store)
	summaries := withQueued.buildTargetSummaries(
		targets, historySample{jobs: []*jobs.Job{queued}, degraded: true}, time.Now(),
	)
	require.Len(t, summaries, 2)
	assert.Equal(t, backupOutcomeUnknown, summaries[0].LastBackupStatus,
		"a target with only an unfinished job in a degraded sample is not 'never'")

	// And the omission has to survive serialization: a present 0 would be read
	// as "nothing failed".
	rr := httptest.NewRecorder()
	dashboardServerWithStore(targets, store).handleDashboard(rr, httptest.NewRequest("GET", "/api/dashboard", nil))

	var raw map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &raw))
	_, present := raw["failed_jobs_24h"]
	assert.False(t, present, "failed_jobs_24h must be omitted, not reported as 0")
}

// TestHandleDashboard_TruncatedHistoryIsUnknown covers the target that scrolls
// out of the capped scan: it stopped being backed up, which must not read the
// same as never having been backed up.
func TestHandleDashboard_TruncatedHistoryIsUnknown(t *testing.T) {
	now := time.Now()

	// One more job than the scan will return, all for one busy target, so the
	// forgotten target's only job falls off the end.
	history := make([]*jobs.Job, 0, dashboardHistoryLimit+1)
	for i := 0; i < dashboardHistoryLimit; i++ {
		history = append(history, backupJob("busy", jobs.JobStatusCompleted,
			now.Add(-time.Duration(i)*time.Second), 64))
	}
	history = append(history, backupJob("forgotten", jobs.JobStatusFailed,
		now.Add(-30*24*time.Hour), 0))

	targets := []*config.Target{dashboardTarget("busy", "*/5 * * * *"), dashboardTarget("forgotten", "0 2 * * *")}
	resp := requestDashboard(t, dashboardServer(targets, history, true))
	require.Len(t, resp.Targets, 2)

	byName := map[string]TargetSummary{}
	for _, summary := range resp.Targets {
		byName[summary.Name] = summary
	}

	assert.Equal(t, backupOutcomeSuccess, byName["busy"].LastBackupStatus)
	assert.Equal(t, backupOutcomeUnknown, byName["forgotten"].LastBackupStatus,
		"a target cut off by the row cap is not a new target")
	assert.False(t, byName["forgotten"].Overdue)

	// The 24h window starts inside the retrieved slice, so it cannot be
	// answered from a sample this truncated either.
	assert.Nil(t, resp.SuccessRate24h)
	assert.Nil(t, resp.FailedJobs24h)
}

// TestHandleDashboard_WindowsNeedProcessUptime covers the storeless daemon
// restarted minutes ago: it has no basis for a 24h figure, and "0 failures in
// the last 24 hours" from a 24-minute-old process is exactly the reassuring
// lie the 7d gate already refuses to tell.
func TestHandleDashboard_WindowsNeedProcessUptime(t *testing.T) {
	resp := requestDashboard(t, dashboardServer(
		[]*config.Target{dashboardTarget("t1", "0 2 * * *")}, nil, false,
	))

	assert.Nil(t, resp.SuccessRate24h)
	assert.Nil(t, resp.FailedJobs24h)
	assert.Nil(t, resp.SuccessRate7d)
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

	// An hourly target whose run takes four minutes: at 12:00 the 12:00 fire is
	// due but the backup for it has not finished yet, and at 12:04 it has.
	hourlyLastSuccess := now.Add(-56 * time.Minute) // 11:04
	twoHoursOfMisses := now.Add(-176 * time.Minute) // 09:04

	tests := []struct {
		name     string
		schedule string
		health   *targetHealth
		running  bool
		want     bool
	}{
		{
			name:     "no schedule is never overdue",
			schedule: "",
			health:   &targetHealth{lastSuccessAt: &twoDaysAgo},
			want:     false,
		},
		{
			name:     "a run due this minute is inside the grace, not an alarm",
			schedule: "0 * * * *",
			health:   &targetHealth{lastSuccessAt: &hourlyLastSuccess},
			want:     false,
		},
		{
			name:     "an hour past the due run is overdue",
			schedule: "0 * * * *",
			health:   &targetHealth{lastSuccessAt: &twoHoursOfMisses},
			want:     true,
		},
		{
			// The grace is capped, so a rare schedule does not get a rare
			// grace: waiting for the second fire here would mean noticing a
			// dead yearly backup in two years.
			name:     "a yearly target is late an hour after its fire, not a year",
			schedule: "0 0 1 1 *",
			health:   &targetHealth{lastSuccessAt: ptr(now.AddDate(-1, 0, 0))},
			want:     true,
		},
		{
			name:     "a yearly target whose fire is still ahead is not late",
			schedule: "0 0 1 1 *",
			health:   &targetHealth{lastSuccessAt: ptr(now.AddDate(0, -1, 0))},
			want:     false,
		},
		{
			name:     "a target with a job in flight is not overdue",
			schedule: "0 * * * *",
			health:   &targetHealth{lastSuccessAt: &twoDaysAgo},
			running:  true,
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
			assert.Equal(t, tt.want, isOverdue(tt.schedule, tt.health, tt.running, now))
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
		degraded   bool
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
		{
			name: "a degraded sample answers nothing, however good it looks",
			history: []*jobs.Job{
				backupJob("t1", jobs.JobStatusCompleted, now.Add(-time.Hour), 1),
			},
			degraded:   true,
			wantRate:   nil,
			wantFailed: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := historySample{jobs: tt.history, truncated: tt.truncated, degraded: tt.degraded}
			rate, failed := backupWindowStats(sample, window24h, now)

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
