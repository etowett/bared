package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"bared/internal/jobs"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLStore(t *testing.T) {
	// Use a temp file for DB
	tmpFile := "test_bared.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLStore("sqlite3", tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create Job
	job := jobs.NewJob(jobs.JobTypeBackup, "test_target", false)
	err = store.CreateJob(ctx, job)
	if err != nil {
		t.Errorf("CreateJob failed: %v", err)
	}

	// 2. Get Job
	fetchedJob, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Errorf("GetJob failed: %v", err)
	}
	if fetchedJob.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, fetchedJob.ID)
	}

	// 3. Update Job
	job.MarkStarted()
	err = store.UpdateJob(ctx, job)
	if err != nil {
		t.Errorf("UpdateJob failed: %v", err)
	}

	fetchedJob, _ = store.GetJob(ctx, job.ID)
	if fetchedJob.Status != jobs.JobStatusRunning {
		t.Errorf("Expected status %s, got %s", jobs.JobStatusRunning, fetchedJob.Status)
	}

	// 4. Locking
	acquired, err := store.AcquireLock(ctx, "lock1", time.Minute)
	if err != nil {
		t.Errorf("AcquireLock failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock")
	}

	// Try to acquire same lock
	acquired2, err := store.AcquireLock(ctx, "lock1", time.Minute)
	if err != nil {
		t.Errorf("AcquireLock failed: %v", err)
	}
	if acquired2 {
		t.Error("Expected NOT to acquire lock")
	}

	// Release
	err = store.ReleaseLock(ctx, "lock1")
	if err != nil {
		t.Errorf("ReleaseLock failed: %v", err)
	}

	// Acquire again
	acquired3, err := store.AcquireLock(ctx, "lock1", time.Minute)
	if err != nil {
		t.Errorf("AcquireLock failed: %v", err)
	}
	if acquired3 != true {
		t.Error("Expected to re-acquire lock")
	}
}
