package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/etowett/bared/apps/api/internal/jobs"

	_ "github.com/mattn/go-sqlite3"
)

func TestSanitizeDSN(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		dsn      string
		expected string
	}{
		{
			name:     "SQLite file path",
			driver:   "sqlite3",
			dsn:      "/var/lib/bared/bared.db",
			expected: "driver=sqlite3, type=file",
		},
		{
			name:     "PostgreSQL URL with credentials",
			driver:   "postgres",
			dsn:      "postgres://user:password@localhost:5432/mydb",
			expected: "driver=postgres, url=postgres://localhost:5432/mydb",
		},
		{
			name:     "PostgreSQL URL with special chars in password",
			driver:   "postgres",
			dsn:      "postgres://admin:p@ssw0rd!@db.example.com:5432/production",
			expected: "driver=postgres, url=postgres://db.example.com:5432/production",
		},
		{
			name:     "MySQL DSN with credentials",
			driver:   "mysql",
			dsn:      "user:pass@tcp(localhost:3306)/dbname",
			expected: "driver=mysql, dsn=<redacted>",
		},
		{
			name:     "DSN with password keyword",
			driver:   "postgres",
			dsn:      "host=localhost port=5432 user=admin password=secret dbname=mydb",
			expected: "driver=postgres, dsn=<redacted>",
		},
		{
			name:     "Generic driver without sensitive data",
			driver:   "customdb",
			dsn:      "localhost:9999",
			expected: "driver=customdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeDSN(tt.driver, tt.dsn)
			if result != tt.expected {
				t.Errorf("SanitizeDSN() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

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
