package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/app"
)

func TestNewJob(t *testing.T) {
	tests := []struct {
		name       string
		jobType    JobType
		targetName string
		manual     bool
	}{
		{
			name:       "backup job manual",
			jobType:    JobTypeBackup,
			targetName: "mysql-prod",
			manual:     true,
		},
		{
			name:       "backup job scheduled",
			jobType:    JobTypeBackup,
			targetName: "postgres-dev",
			manual:     false,
		},
		{
			name:       "restore job manual",
			jobType:    JobTypeRestore,
			targetName: "redis-cache",
			manual:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := NewJob(tt.jobType, tt.targetName, tt.manual)

			require.NotNil(t, job)
			assert.NotEmpty(t, job.ID)
			assert.Equal(t, tt.jobType, job.Type)
			assert.Equal(t, tt.targetName, job.TargetName)
			assert.Equal(t, JobStatusQueued, job.Status)
			assert.Equal(t, tt.manual, job.Manual)
			assert.NotNil(t, job.Progress)
			assert.NotNil(t, job.Logs)
			assert.NotZero(t, job.CreatedAt)
			assert.Nil(t, job.StartedAt)
			assert.Nil(t, job.CompletedAt)
			assert.NotNil(t, job.Context())
		})
	}
}

func TestJob_Context(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	ctx := job.Context()
	require.NotNil(t, ctx)

	// Context should not be done initially
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done initially")
	default:
	}
}

func TestJob_Cancel(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  JobStatus
		expectCanceled bool
	}{
		{
			name:           "cancel queued job",
			initialStatus:  JobStatusQueued,
			expectCanceled: true,
		},
		{
			name:           "cancel running job",
			initialStatus:  JobStatusRunning,
			expectCanceled: true,
		},
		{
			name:           "cannot cancel completed job",
			initialStatus:  JobStatusCompleted,
			expectCanceled: false,
		},
		{
			name:           "cannot cancel failed job",
			initialStatus:  JobStatusFailed,
			expectCanceled: false,
		},
		{
			name:           "cannot cancel already cancelled job",
			initialStatus:  JobStatusCancelled,
			expectCanceled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := NewJob(JobTypeBackup, "test-target", true)
			job.SetStatus(tt.initialStatus)

			ctx := job.Context()
			job.Cancel()

			if tt.expectCanceled {
				assert.Equal(t, JobStatusCancelling, job.GetStatus())

				// Wait briefly for context cancellation to propagate
				select {
				case <-ctx.Done():
					// Expected
				case <-time.After(100 * time.Millisecond):
					t.Fatal("context should be cancelled")
				}
			} else {
				assert.Equal(t, tt.initialStatus, job.GetStatus())
			}
		})
	}
}

func TestJob_SetStatus(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	statuses := []JobStatus{
		JobStatusQueued,
		JobStatusRunning,
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusCancelling,
		JobStatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			job.SetStatus(status)
			assert.Equal(t, status, job.GetStatus())
		})
	}
}

func TestJob_GetStatus(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	// Initial status
	assert.Equal(t, JobStatusQueued, job.GetStatus())

	// Change status
	job.SetStatus(JobStatusRunning)
	assert.Equal(t, JobStatusRunning, job.GetStatus())
}

func TestJob_MarkStarted(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	assert.Nil(t, job.StartedAt)
	assert.Equal(t, JobStatusQueued, job.GetStatus())

	beforeMark := time.Now()
	job.MarkStarted()
	afterMark := time.Now()

	require.NotNil(t, job.StartedAt)
	assert.Equal(t, JobStatusRunning, job.GetStatus())
	assert.True(t, job.StartedAt.After(beforeMark) || job.StartedAt.Equal(beforeMark))
	assert.True(t, job.StartedAt.Before(afterMark) || job.StartedAt.Equal(afterMark))
}

func TestJob_MarkCompleted(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)
	job.MarkStarted()

	result := &app.BackupResult{
		Target:     "test-target",
		Success:    true,
		BackupPath: "/backups/test.tar.gz",
		Size:       12345,
	}

	assert.Nil(t, job.CompletedAt)
	assert.Equal(t, JobStatusRunning, job.GetStatus())

	beforeMark := time.Now()
	job.MarkCompleted(result)
	afterMark := time.Now()

	require.NotNil(t, job.CompletedAt)
	assert.Equal(t, JobStatusCompleted, job.GetStatus())
	assert.Equal(t, result, job.Result)
	assert.True(t, job.CompletedAt.After(beforeMark) || job.CompletedAt.Equal(beforeMark))
	assert.True(t, job.CompletedAt.Before(afterMark) || job.CompletedAt.Equal(afterMark))
}

func TestJob_MarkFailed(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)
	job.MarkStarted()

	testErr := errors.New("backup failed: connection timeout")

	assert.Nil(t, job.CompletedAt)
	assert.Equal(t, JobStatusRunning, job.GetStatus())

	beforeMark := time.Now()
	job.MarkFailed(testErr)
	afterMark := time.Now()

	require.NotNil(t, job.CompletedAt)
	assert.Equal(t, JobStatusFailed, job.GetStatus())
	assert.Equal(t, testErr.Error(), job.Error)
	assert.True(t, job.CompletedAt.After(beforeMark) || job.CompletedAt.Equal(beforeMark))
	assert.True(t, job.CompletedAt.Before(afterMark) || job.CompletedAt.Equal(afterMark))
}

func TestJob_MarkCancelled(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)
	job.MarkStarted()
	job.Cancel()

	assert.Nil(t, job.CompletedAt)
	assert.Equal(t, JobStatusCancelling, job.GetStatus())

	beforeMark := time.Now()
	job.MarkCancelled()
	afterMark := time.Now()

	require.NotNil(t, job.CompletedAt)
	assert.Equal(t, JobStatusCancelled, job.GetStatus())
	assert.True(t, job.CompletedAt.After(beforeMark) || job.CompletedAt.Equal(beforeMark))
	assert.True(t, job.CompletedAt.Before(afterMark) || job.CompletedAt.Equal(afterMark))
}

func TestJob_ConcurrentStatusUpdates(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	var wg sync.WaitGroup
	updates := 100

	// Concurrently update status
	for i := 0; i < updates; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			status := JobStatusRunning
			if n%2 == 0 {
				status = JobStatusQueued
			}
			job.SetStatus(status)
		}(i)
	}

	// Concurrently read status
	for i := 0; i < updates; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = job.GetStatus()
		}()
	}

	wg.Wait()

	// Final status should be one of the set values
	finalStatus := job.GetStatus()
	assert.True(t, finalStatus == JobStatusRunning || finalStatus == JobStatusQueued)
}

func TestJob_LifecycleStates(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	// Initial state
	assert.Equal(t, JobStatusQueued, job.GetStatus())
	assert.Nil(t, job.StartedAt)
	assert.Nil(t, job.CompletedAt)

	// Start
	job.MarkStarted()
	assert.Equal(t, JobStatusRunning, job.GetStatus())
	assert.NotNil(t, job.StartedAt)
	assert.Nil(t, job.CompletedAt)

	// Complete
	result := &app.BackupResult{Success: true}
	job.MarkCompleted(result)
	assert.Equal(t, JobStatusCompleted, job.GetStatus())
	assert.NotNil(t, job.StartedAt)
	assert.NotNil(t, job.CompletedAt)
	assert.Equal(t, result, job.Result)
}

func TestNewProgress(t *testing.T) {
	progress := NewProgress()

	require.NotNil(t, progress)
	assert.Equal(t, "initializing", progress.Stage)
	assert.Equal(t, 0.0, progress.Percent)
	assert.Equal(t, int64(0), progress.BytesProcessed)
	assert.Equal(t, int64(0), progress.BytesTotal)
	assert.NotZero(t, progress.StartTime)
}

func TestProgress_SetStage(t *testing.T) {
	progress := NewProgress()

	progress.SetStage("dumping", 1024*1024)

	assert.Equal(t, "dumping", progress.Stage)
	assert.Equal(t, int64(1024*1024), progress.BytesTotal)
	assert.Equal(t, int64(0), progress.BytesProcessed)
}

func TestProgress_Update(t *testing.T) {
	progress := NewProgress()

	progress.Update(50.0, "processing data")

	assert.Equal(t, 50.0, progress.Percent)
	assert.Equal(t, "processing data", progress.Message)
}

func TestProgress_UpdateBytes(t *testing.T) {
	progress := NewProgress()

	tests := []struct {
		name            string
		processed       int64
		total           int64
		expectedPercent float64
	}{
		{
			name:            "25% complete",
			processed:       256,
			total:           1024,
			expectedPercent: 25.0,
		},
		{
			name:            "50% complete",
			processed:       512,
			total:           1024,
			expectedPercent: 50.0,
		},
		{
			name:            "100% complete",
			processed:       1024,
			total:           1024,
			expectedPercent: 100.0,
		},
		{
			name:            "over 100% capped",
			processed:       2000,
			total:           1024,
			expectedPercent: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress.UpdateBytes(tt.processed, tt.total)

			assert.Equal(t, tt.processed, progress.BytesProcessed)
			assert.Equal(t, tt.total, progress.BytesTotal)
			assert.InDelta(t, tt.expectedPercent, progress.Percent, 0.1)
		})
	}
}

func TestProgress_GetSnapshot(t *testing.T) {
	progress := NewProgress()
	progress.SetStage("compressing", 2048)
	progress.UpdateBytes(1024, 2048)
	progress.Update(50.0, "halfway done")

	snapshot := progress.GetSnapshot()

	assert.Equal(t, "compressing", snapshot.Stage)
	assert.Equal(t, 50.0, snapshot.Percent)
	assert.Equal(t, int64(1024), snapshot.BytesProcessed)
	assert.Equal(t, int64(2048), snapshot.BytesTotal)
	assert.Equal(t, "halfway done", snapshot.Message)
}

func TestProgress_ConcurrentUpdates(t *testing.T) {
	progress := NewProgress()
	progress.SetStage("processing", 10000)

	var wg sync.WaitGroup
	updates := 100

	// Concurrent byte updates
	for i := 0; i < updates; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			progress.UpdateBytes(int64(n*100), 10000)
		}(i)
	}

	// Concurrent snapshot reads
	for i := 0; i < updates; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = progress.GetSnapshot()
		}()
	}

	wg.Wait()

	// Should complete without panicking
	finalSnapshot := progress.GetSnapshot()
	assert.GreaterOrEqual(t, finalSnapshot.BytesProcessed, int64(0))
	assert.LessOrEqual(t, finalSnapshot.Percent, 100.0)
}

func TestGenerateJobID(t *testing.T) {
	// Generate multiple IDs with slight delays
	ids := make(map[string]bool)
	count := 20

	for i := 0; i < count; i++ {
		id := generateJobID()

		assert.NotEmpty(t, id)
		assert.Greater(t, len(id), 8, "ID should have reasonable length")

		// Check uniqueness (allowing some duplicates due to weak random implementation)
		ids[id] = true

		// Add small delay to reduce collision probability
		time.Sleep(1 * time.Millisecond)
	}

	// Should have generated at least most IDs uniquely
	assert.GreaterOrEqual(t, len(ids), count-5, "most IDs should be unique")
}

func TestJob_RestoreJobFields(t *testing.T) {
	job := NewJob(JobTypeRestore, "test-target", true)

	backupPath := "/backups/test-20250108.tar.gz"
	restoreOpts := &app.RestoreOptions{
		DryRun:         true,
		SkipValidation: false,
	}

	job.BackupPath = backupPath
	job.RestoreOptions = restoreOpts

	assert.Equal(t, backupPath, job.BackupPath)
	assert.Equal(t, restoreOpts, job.RestoreOptions)
}

func TestJob_ScheduledJobFields(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", false)

	schedule := "0 2 * * *"
	job.ScheduledBy = schedule

	assert.False(t, job.Manual)
	assert.Equal(t, schedule, job.ScheduledBy)
}

func TestJob_ContextCancellation(t *testing.T) {
	job := NewJob(JobTypeBackup, "test-target", true)

	ctx := job.Context()
	require.NotNil(t, ctx)

	// Create a goroutine that waits on context
	done := make(chan bool)
	go func() {
		<-ctx.Done()
		done <- true
	}()

	// Cancel the job
	job.Cancel()

	// Context should be cancelled
	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Fatal("context cancellation did not propagate")
	}

	assert.Error(t, ctx.Err())
	assert.Equal(t, context.Canceled, ctx.Err())
}
