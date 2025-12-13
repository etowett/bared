// Package persistence implements the storage layer for various backends.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bared/internal/jobs"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLStore implements JobStore using database/sql
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore creates a new SQL store
func NewSQLStore(driver, dsn string) (*SQLStore, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	store := &SQLStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *SQLStore) initSchema() error {
	// Simple schema for SQLite/Postgres compatibility
	// Note: In production, use migrations. This is a simplified approach.

	createJobsTable := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		target_name TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		result_json TEXT,
		error TEXT,
		manual BOOLEAN,
		schedule TEXT
	);`

	createLocksTable := `
	CREATE TABLE IF NOT EXISTS locks (
		name TEXT PRIMARY KEY,
		holder TEXT,
		expires_at DATETIME NOT NULL
	);`

	// DB specific syntax adjustments could be handled here

	if _, err := s.db.Exec(createJobsTable); err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}
	if _, err := s.db.Exec(createLocksTable); err != nil {
		return fmt.Errorf("failed to create locks table: %w", err)
	}

	return nil
}

func (s *SQLStore) CreateJob(ctx context.Context, job *jobs.Job) error {
	query := `INSERT INTO jobs (id, type, target_name, status, created_at, manual) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, job.ID, job.Type, job.TargetName, job.Status, job.CreatedAt, job.Manual)
	return err
}

func (s *SQLStore) UpdateJob(ctx context.Context, job *jobs.Job) error {
	// Serialize result if present
	var resultJSON []byte
	if job.Result != nil {
		var err error
		resultJSON, err = json.Marshal(job.Result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
	}

	var errStr string
	if job.Error != nil {
		errStr = job.Error.Error()
	}

	query := `
		UPDATE jobs
		SET status=?, started_at=?, completed_at=?, result_json=?, error=?
		WHERE id=?
	`
	_, err := s.db.ExecContext(ctx, query,
		job.Status, job.StartedAt, job.CompletedAt, string(resultJSON), errStr, job.ID)
	return err
}

func (s *SQLStore) GetJob(ctx context.Context, id jobs.JobID) (*jobs.Job, error) {
	query := `SELECT id, type, target_name, status, created_at, started_at, completed_at, result_json, error, manual, schedule FROM jobs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var job jobs.Job
	var startedAt, completedAt sql.NullTime
	var resultJSON, errorStr sql.NullString
	var schedule sql.NullString
	var idStr string
	var typeStr string
	var statusStr string

	err := row.Scan(&idStr, &typeStr, &job.TargetName, &statusStr, &job.CreatedAt, &startedAt, &completedAt, &resultJSON, &errorStr, &job.Manual, &schedule)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, err
	}

	job.ID = jobs.JobID(idStr)
	job.Type = jobs.JobType(typeStr)
	job.Status = jobs.JobStatus(statusStr)

	if startedAt.Valid {
		t := startedAt.Time
		job.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		job.CompletedAt = &t
	}
	if errorStr.Valid && errorStr.String != "" {
		job.Error = errors.New(errorStr.String)
	}
	if schedule.Valid {
		job.ScheduledBy = schedule.String
	}

	// Note: Unmarshalling result is tricky without knowing specific type (BackupResult/RestoreResult).
	// For now, we might skip it or unmarshal into map[string]interface{}
	// Or simplistic:
	// if resultJSON.Valid { ... }

	return &job, nil
}

func (s *SQLStore) ListJobs(ctx context.Context, filter jobs.JobFilter) ([]*jobs.Job, error) {
	// Basic implementation
	query := `SELECT id, type, target_name, status, created_at, started_at, completed_at, result_json, error, manual FROM jobs ORDER BY created_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, filter.Limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			_ = err
		}
	}()

	var result []*jobs.Job
	for rows.Next() {
		var job jobs.Job
		var startedAt, completedAt sql.NullTime
		var resultJSON, errorStr sql.NullString
		var idStr, typeStr, statusStr string

		if err := rows.Scan(&idStr, &typeStr, &job.TargetName, &statusStr, &job.CreatedAt, &startedAt, &completedAt, &resultJSON, &errorStr, &job.Manual); err != nil {
			return nil, err
		}

		job.ID = jobs.JobID(idStr)
		job.Type = jobs.JobType(typeStr)
		job.Status = jobs.JobStatus(statusStr)
		if startedAt.Valid {
			t := startedAt.Time
			job.StartedAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			job.CompletedAt = &t
		}
		// Skip full reconstruction for list to save perf

		result = append(result, &job)
	}
	return result, nil
}

func (s *SQLStore) AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error) {
	// Clean up expired locks first (lazy cleanup)
	// In production, use a background reaper
	if _, err := s.db.Exec(`DELETE FROM locks WHERE expires_at < ?`, time.Now()); err != nil {
		// Ignore cleanup error
		_ = err
	}

	expiresAt := time.Now().Add(ttl)

	// Create or fail
	query := `INSERT INTO locks (name, holder, expires_at) VALUES (?, 'instance', ?)`
	_, err := s.db.ExecContext(ctx, query, lockName, expiresAt)
	if err != nil {
		// Assume violation means locked
		return false, nil
	}
	return true, nil
}

func (s *SQLStore) ReleaseLock(ctx context.Context, lockName string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM locks WHERE name = ?`, lockName)
	return err
}

func (s *SQLStore) Close() error {
	return s.db.Close()
}
