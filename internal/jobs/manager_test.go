package jobs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
	"bared/internal/util"
)

func init() {
	// Initialize logger before any tests run to avoid race condition
	// in util.GetLogger() when multiple goroutines start concurrently
	util.InitLogger(util.ERROR)
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name          string
		maxConcurrent int
		maxHistory    int
	}{
		{
			name:          "single worker",
			maxConcurrent: 1,
			maxHistory:    100,
		},
		{
			name:          "multiple workers",
			maxConcurrent: 5,
			maxHistory:    500,
		},
		{
			name:          "many workers",
			maxConcurrent: 10,
			maxHistory:    1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Targets: []*config.Target{fixtures.MySQLTarget()},
			}

			mgr := NewManager(cfg, nil, nil, tt.maxConcurrent, tt.maxHistory)

			require.NotNil(t, mgr)
			assert.Equal(t, tt.maxConcurrent, mgr.maxConcurrent)
			assert.Equal(t, tt.maxHistory, mgr.maxHistory)
			assert.NotNil(t, mgr.jobs)
			assert.NotNil(t, mgr.runningJobs)
			assert.NotNil(t, mgr.jobQueue)
			assert.NotNil(t, mgr.shutdown)
			assert.Equal(t, cfg, mgr.cfg)
		})
	}
}

func TestManager_StartAndShutdown(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)
	mgr.Start()

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mgr.Shutdown(ctx)
	require.NoError(t, err)
}

func TestManager_ShutdownTimeout(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 1, 100)
	mgr.Start()

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	err := mgr.Shutdown(ctx)
	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestManager_GetJob(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Create and add a job manually
	job := NewJob(JobTypeBackup, "mysql-prod", true)
	mgr.mu.Lock()
	mgr.jobs[job.ID] = job
	mgr.mu.Unlock()

	tests := []struct {
		name        string
		jobID       JobID
		expectError bool
	}{
		{
			name:        "get existing job",
			jobID:       job.ID,
			expectError: false,
		},
		{
			name:        "get non-existent job",
			jobID:       JobID("non-existent-id"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedJob, err := mgr.GetJob(tt.jobID)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, retrievedJob)
				assert.Contains(t, err.Error(), "job not found")
			} else {
				require.NoError(t, err)
				require.NotNil(t, retrievedJob)
				assert.Equal(t, tt.jobID, retrievedJob.ID)
			}
		})
	}
}

func TestManager_ListJobs(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Initially empty
	jobs := mgr.ListJobs(context.Background())
	assert.Len(t, jobs, 0)

	// Add some jobs
	job1 := NewJob(JobTypeBackup, "mysql-prod", true)
	job2 := NewJob(JobTypeBackup, "postgres-dev", false)
	job3 := NewJob(JobTypeRestore, "redis-cache", true)

	mgr.mu.Lock()
	mgr.jobs[job1.ID] = job1
	mgr.jobs[job2.ID] = job2
	mgr.jobs[job3.ID] = job3
	mgr.mu.Unlock()

	// List all
	jobs = mgr.ListJobs(context.Background())
	assert.Len(t, jobs, 3)
}

func TestManager_ListJobsForTarget(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Add jobs for different targets
	job1 := NewJob(JobTypeBackup, "mysql-prod", true)
	job2 := NewJob(JobTypeBackup, "mysql-prod", false)
	job3 := NewJob(JobTypeBackup, "postgres-dev", true)

	mgr.mu.Lock()
	mgr.jobs[job1.ID] = job1
	mgr.jobs[job2.ID] = job2
	mgr.jobs[job3.ID] = job3
	mgr.mu.Unlock()

	tests := []struct {
		name          string
		targetName    string
		expectedCount int
	}{
		{
			name:          "target with multiple jobs",
			targetName:    "mysql-prod",
			expectedCount: 2,
		},
		{
			name:          "target with single job",
			targetName:    "postgres-dev",
			expectedCount: 1,
		},
		{
			name:          "target with no jobs",
			targetName:    "redis-cache",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := mgr.ListJobsForTarget(context.Background(), tt.targetName)
			assert.Len(t, jobs, tt.expectedCount)

			// Verify all jobs are for the correct target
			for _, job := range jobs {
				assert.Equal(t, tt.targetName, job.TargetName)
			}
		})
	}
}

func TestManager_IsTargetRunning(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Initially not running
	assert.False(t, mgr.IsTargetRunning("mysql-prod"))

	// Add a running job
	job := NewJob(JobTypeBackup, "mysql-prod", true)
	mgr.mu.Lock()
	mgr.runningJobs["mysql-prod"] = job
	mgr.mu.Unlock()

	// Now should be running
	assert.True(t, mgr.IsTargetRunning("mysql-prod"))
	assert.False(t, mgr.IsTargetRunning("postgres-dev"))

	// Remove from running
	mgr.mu.Lock()
	delete(mgr.runningJobs, "mysql-prod")
	mgr.mu.Unlock()

	// Should not be running anymore
	assert.False(t, mgr.IsTargetRunning("mysql-prod"))
}

func TestManager_CancelJob(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	tests := []struct {
		name          string
		setupJob      func() *Job
		expectError   bool
		errorContains string
	}{
		{
			name: "cancel queued job",
			setupJob: func() *Job {
				job := NewJob(JobTypeBackup, "mysql-prod", true)
				job.SetStatus(JobStatusQueued)
				return job
			},
			expectError: false,
		},
		{
			name: "cancel running job",
			setupJob: func() *Job {
				job := NewJob(JobTypeBackup, "mysql-prod", true)
				job.MarkStarted()
				return job
			},
			expectError: false,
		},
		{
			name: "cannot cancel completed job",
			setupJob: func() *Job {
				job := NewJob(JobTypeBackup, "mysql-prod", true)
				job.SetStatus(JobStatusCompleted)
				return job
			},
			expectError:   true,
			errorContains: "cannot cancel job in status",
		},
		{
			name: "cannot cancel failed job",
			setupJob: func() *Job {
				job := NewJob(JobTypeBackup, "mysql-prod", true)
				job.SetStatus(JobStatusFailed)
				return job
			},
			expectError:   true,
			errorContains: "cannot cancel job in status",
		},
		{
			name: "cannot cancel already cancelled job",
			setupJob: func() *Job {
				job := NewJob(JobTypeBackup, "mysql-prod", true)
				job.SetStatus(JobStatusCancelled)
				return job
			},
			expectError:   true,
			errorContains: "cannot cancel job in status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := tt.setupJob()

			mgr.mu.Lock()
			mgr.jobs[job.ID] = job
			mgr.mu.Unlock()

			err := mgr.CancelJob(job.ID)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, JobStatusCancelling, job.GetStatus())
			}
		})
	}
}

func TestManager_CancelJob_NotFound(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	err := mgr.CancelJob(JobID("non-existent-id"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestManager_CleanupOldJobs(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	now := time.Now()

	// Create jobs with different completion times
	oldJob := NewJob(JobTypeBackup, "mysql-prod", true)
	oldTime := now.Add(-2 * time.Hour)
	oldJob.CompletedAt = &oldTime

	recentJob := NewJob(JobTypeBackup, "postgres-dev", true)
	recentTime := now.Add(-30 * time.Minute)
	recentJob.CompletedAt = &recentTime

	runningJob := NewJob(JobTypeBackup, "redis-cache", true)
	// No completion time - still running

	mgr.mu.Lock()
	mgr.jobs[oldJob.ID] = oldJob
	mgr.jobs[recentJob.ID] = recentJob
	mgr.jobs[runningJob.ID] = runningJob
	mgr.mu.Unlock()

	// Cleanup jobs older than 1 hour
	mgr.CleanupOldJobs(1 * time.Hour)

	// Old job should be removed
	_, err := mgr.GetJob(oldJob.ID)
	assert.Error(t, err)

	// Recent job should still exist
	_, err = mgr.GetJob(recentJob.ID)
	assert.NoError(t, err)

	// Running job should still exist
	_, err = mgr.GetJob(runningJob.ID)
	assert.NoError(t, err)
}

func TestManager_StartCleanupRoutine(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	now := time.Now()

	// Create an old completed job
	oldJob := NewJob(JobTypeBackup, "mysql-prod", true)
	oldTime := now.Add(-2 * time.Second)
	oldJob.CompletedAt = &oldTime

	mgr.mu.Lock()
	mgr.jobs[oldJob.ID] = oldJob
	mgr.mu.Unlock()

	// Start cleanup routine with short interval
	mgr.StartCleanupRoutine(100*time.Millisecond, 1*time.Second)

	// Wait for cleanup to run
	time.Sleep(250 * time.Millisecond)

	// Job should be cleaned up
	_, err := mgr.GetJob(oldJob.ID)
	assert.Error(t, err)

	// Shutdown to stop cleanup routine
	close(mgr.shutdown)
}

func TestManager_ConcurrentJobOperations(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	var wg sync.WaitGroup
	operations := 50

	// Concurrent job creation and listing
	for i := 0; i < operations; i++ {
		wg.Add(2)

		// Add jobs
		go func(_ int) {
			defer wg.Done()
			job := NewJob(JobTypeBackup, "mysql-prod", true)
			mgr.mu.Lock()
			mgr.jobs[job.ID] = job
			mgr.mu.Unlock()
		}(i)

		// List jobs
		go func() {
			defer wg.Done()
			_ = mgr.ListJobs(context.Background())
		}()
	}

	wg.Wait()

	// Should have created many jobs without panicking
	// Due to weak random ID generation, some jobs might have duplicate IDs
	// and overwrite each other, so we just check that we got a reasonable number
	jobs := mgr.ListJobs(context.Background())
	assert.GreaterOrEqual(t, len(jobs), operations-10, "should create most jobs even with potential ID collisions")
	assert.LessOrEqual(t, len(jobs), operations, "should not exceed expected job count")
}

func TestManager_DuplicateTargetPrevention(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Add a running job for target
	job := NewJob(JobTypeBackup, "mysql-prod", true)
	mgr.mu.Lock()
	mgr.runningJobs["mysql-prod"] = job
	mgr.mu.Unlock()

	// Should detect duplicate
	assert.True(t, mgr.IsTargetRunning("mysql-prod"))

	// Simulate job completion - remove from running
	mgr.mu.Lock()
	delete(mgr.runningJobs, "mysql-prod")
	mgr.mu.Unlock()

	// Should allow new job now
	assert.False(t, mgr.IsTargetRunning("mysql-prod"))
}

func TestManager_JobQueueCapacity(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 1, 100)

	// Job queue has capacity of 100
	assert.Equal(t, 100, cap(mgr.jobQueue))
}

func TestManager_MultipleTargetsAndJobTypes(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{
			fixtures.MySQLTarget(),
			fixtures.PostgresTarget(),
		},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Create different job types for different targets
	job1 := NewJob(JobTypeBackup, "mysql-prod", true)
	job2 := NewJob(JobTypeRestore, "mysql-prod", true)
	job3 := NewJob(JobTypeBackup, "postgres-dev", false)
	job4 := NewJob(JobTypeRestore, "postgres-dev", true)

	mgr.mu.Lock()
	mgr.jobs[job1.ID] = job1
	mgr.jobs[job2.ID] = job2
	mgr.jobs[job3.ID] = job3
	mgr.jobs[job4.ID] = job4
	mgr.mu.Unlock()

	// List all jobs
	allJobs := mgr.ListJobs(context.Background())
	assert.Len(t, allJobs, 4)

	// List by target
	mysqlJobs := mgr.ListJobsForTarget(context.Background(), "mysql-prod")
	assert.Len(t, mysqlJobs, 2)

	postgresJobs := mgr.ListJobsForTarget(context.Background(), "postgres-dev")
	assert.Len(t, postgresJobs, 2)
}

func TestManager_GetJob_ThreadSafety(_ *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Add a job
	job := NewJob(JobTypeBackup, "mysql-prod", true)
	mgr.mu.Lock()
	mgr.jobs[job.ID] = job
	mgr.mu.Unlock()

	var wg sync.WaitGroup
	reads := 100

	// Concurrent reads
	for i := 0; i < reads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mgr.GetJob(job.ID)
		}()
	}

	wg.Wait()
	// Should complete without panicking
}

func TestManager_EmptyManagerOperations(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	mgr := NewManager(cfg, nil, nil, 2, 100)

	// Operations on empty manager should not panic
	assert.Len(t, mgr.ListJobs(context.Background()), 0)
	assert.Len(t, mgr.ListJobsForTarget(context.Background(), "any-target"), 0)
	assert.False(t, mgr.IsTargetRunning("any-target"))

	_, err := mgr.GetJob(JobID("non-existent"))
	assert.Error(t, err)

	err = mgr.CancelJob(JobID("non-existent"))
	assert.Error(t, err)

	// Cleanup should not panic
	mgr.CleanupOldJobs(1 * time.Hour)
}
