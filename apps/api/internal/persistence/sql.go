// Package persistence implements the storage layer for various backends.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/etowett/bared/apps/api/internal/jobs"
	"github.com/etowett/bared/apps/api/internal/util"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLStore implements JobStore using database/sql
type SQLStore struct {
	db *sql.DB
}

// SanitizeDSN removes credentials from DSN for safe logging
func SanitizeDSN(driver, dsn string) string {
	// For SQLite, DSN is typically just a file path
	if driver == "sqlite3" {
		return fmt.Sprintf("driver=%s, type=file", driver)
	}

	// Try to parse as URL (postgres://user:pass@host:port/db)
	// Only treat as URL if it has a recognized database scheme
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		dbSchemes := map[string]bool{
			"postgres":   true,
			"postgresql": true,
			"mysql":      true,
			"mongodb":    true,
			"redis":      true,
		}

		if dbSchemes[u.Scheme] {
			// Remove user info (username and password)
			u.User = nil
			return fmt.Sprintf("driver=%s, url=%s", driver, u.String())
		}
	}

	// Check if DSN contains sensitive keywords or @ symbol
	// (for non-URL formats like "user:pass@tcp(host)/db" or "password=...")
	if strings.Contains(dsn, "password") || strings.Contains(dsn, "@") {
		return fmt.Sprintf("driver=%s, dsn=<redacted>", driver)
	}

	// If no sensitive data detected, log basic info
	return fmt.Sprintf("driver=%s", driver)
}

// NewSQLStore creates a new SQL store
func NewSQLStore(driver, dsn string) (*SQLStore, error) {
	logger := util.GetLogger()
	logger.InfoS("Opening database connection",
		"component", "persistence",
		"connection_info", SanitizeDSN(driver, dsn))

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &SQLStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
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

	createJobLogsTable := `
	CREATE TABLE IF NOT EXISTS job_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		stage TEXT,
		FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
	);`

	createJobLogsIndexes := `
	CREATE INDEX IF NOT EXISTS idx_job_logs_job_id ON job_logs(job_id);
	CREATE INDEX IF NOT EXISTS idx_job_logs_timestamp ON job_logs(timestamp);`

	// Config management tables (added for API-based configuration)

	createEncryptionKeysTable := `
	CREATE TABLE IF NOT EXISTS encryption_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_data TEXT NOT NULL,
		active BOOLEAN NOT NULL DEFAULT true,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	createStoragesTable := `
	CREATE TABLE IF NOT EXISTS storages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL,
		config_json TEXT NOT NULL,
		keep INTEGER NOT NULL DEFAULT 7,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	createStoragesIndexes := `
	CREATE INDEX IF NOT EXISTS idx_storages_name ON storages(name);
	CREATE INDEX IF NOT EXISTS idx_storages_enabled ON storages(enabled);`

	createNotifiersTable := `
	CREATE TABLE IF NOT EXISTS notifiers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL,
		config_json TEXT NOT NULL,
		on_success BOOLEAN NOT NULL DEFAULT false,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	createNotifiersIndexes := `
	CREATE INDEX IF NOT EXISTS idx_notifiers_name ON notifiers(name);
	CREATE INDEX IF NOT EXISTS idx_notifiers_enabled ON notifiers(enabled);`

	createTargetsTable := `
	CREATE TABLE IF NOT EXISTS targets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL,
		conn_type TEXT NOT NULL,
		conn_json TEXT NOT NULL,
		storage_name TEXT,
		schedule TEXT,
		compress_enabled BOOLEAN NOT NULL DEFAULT false,
		compress_type TEXT,
		exclude_tables TEXT,
		additional_args TEXT,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (storage_name) REFERENCES storages(name) ON DELETE SET NULL
	);`

	createTargetsIndexes := `
	CREATE INDEX IF NOT EXISTS idx_targets_name ON targets(name);
	CREATE INDEX IF NOT EXISTS idx_targets_enabled ON targets(enabled);
	CREATE INDEX IF NOT EXISTS idx_targets_schedule ON targets(schedule) WHERE schedule IS NOT NULL;`

	createRestoreTargetsTable := `
	CREATE TABLE IF NOT EXISTS restore_targets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		conn_type TEXT NOT NULL,
		conn_json TEXT NOT NULL,
		storage_name TEXT,
		source_target TEXT,
		description TEXT,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (storage_name) REFERENCES storages(name) ON DELETE SET NULL,
		FOREIGN KEY (source_target) REFERENCES targets(name) ON DELETE SET NULL
	);`

	createRestoreTargetsIndexes := `
	CREATE INDEX IF NOT EXISTS idx_restore_targets_name ON restore_targets(name);
	CREATE INDEX IF NOT EXISTS idx_restore_targets_enabled ON restore_targets(enabled);`

	createGlobalConfigTable := `
	CREATE TABLE IF NOT EXISTS global_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	//nolint:gosec // G101: false positive - this is a SQL table definition, not hardcoded credentials
	createSecretsTable := `
	CREATE TABLE IF NOT EXISTS secrets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ref_type TEXT NOT NULL,
		ref_id INTEGER NOT NULL,
		field_name TEXT NOT NULL,
		encrypted_value TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(ref_type, ref_id, field_name)
	);`

	//nolint:gosec // G101: false positive - this is a SQL index definition
	createSecretsIndexes := `
	CREATE INDEX IF NOT EXISTS idx_secrets_ref ON secrets(ref_type, ref_id);`

	initGlobalConfig := `
	INSERT OR IGNORE INTO global_config (key, value) VALUES
		('default_storage', ''),
		('log_level', 'info'),
		('log_format', 'auto');`

	// DB specific syntax adjustments could be handled here

	// Execute original tables
	if _, err := s.db.Exec(createJobsTable); err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}
	if _, err := s.db.Exec(createLocksTable); err != nil {
		return fmt.Errorf("failed to create locks table: %w", err)
	}
	if _, err := s.db.Exec(createJobLogsTable); err != nil {
		return fmt.Errorf("failed to create job_logs table: %w", err)
	}
	if _, err := s.db.Exec(createJobLogsIndexes); err != nil {
		return fmt.Errorf("failed to create job_logs indexes: %w", err)
	}

	// Execute config management tables
	if _, err := s.db.Exec(createEncryptionKeysTable); err != nil {
		return fmt.Errorf("failed to create encryption_keys table: %w", err)
	}
	if _, err := s.db.Exec(createStoragesTable); err != nil {
		return fmt.Errorf("failed to create storages table: %w", err)
	}
	if _, err := s.db.Exec(createStoragesIndexes); err != nil {
		return fmt.Errorf("failed to create storages indexes: %w", err)
	}
	if _, err := s.db.Exec(createNotifiersTable); err != nil {
		return fmt.Errorf("failed to create notifiers table: %w", err)
	}
	if _, err := s.db.Exec(createNotifiersIndexes); err != nil {
		return fmt.Errorf("failed to create notifiers indexes: %w", err)
	}
	if _, err := s.db.Exec(createTargetsTable); err != nil {
		return fmt.Errorf("failed to create targets table: %w", err)
	}
	if _, err := s.db.Exec(createTargetsIndexes); err != nil {
		return fmt.Errorf("failed to create targets indexes: %w", err)
	}
	if _, err := s.db.Exec(createRestoreTargetsTable); err != nil {
		return fmt.Errorf("failed to create restore_targets table: %w", err)
	}
	if _, err := s.db.Exec(createRestoreTargetsIndexes); err != nil {
		return fmt.Errorf("failed to create restore_targets indexes: %w", err)
	}
	if _, err := s.db.Exec(createGlobalConfigTable); err != nil {
		return fmt.Errorf("failed to create global_config table: %w", err)
	}
	if _, err := s.db.Exec(createSecretsTable); err != nil {
		return fmt.Errorf("failed to create secrets table: %w", err)
	}
	if _, err := s.db.Exec(createSecretsIndexes); err != nil {
		return fmt.Errorf("failed to create secrets indexes: %w", err)
	}
	if _, err := s.db.Exec(initGlobalConfig); err != nil {
		return fmt.Errorf("failed to initialize global_config: %w", err)
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

	query := `
		UPDATE jobs
		SET status=?, started_at=?, completed_at=?, result_json=?, error=?
		WHERE id=?
	`
	_, err := s.db.ExecContext(ctx, query,
		job.Status, job.StartedAt, job.CompletedAt, string(resultJSON), job.Error, job.ID)
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
		job.Error = errorStr.String
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
	// Filtered + ordered listing (newest first).
	// Note: We intentionally keep this "basic" (no joins, minimal columns) for performance.
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 6)

	if filter.TargetName != "" {
		conditions = append(conditions, "target_name = ?")
		args = append(args, filter.TargetName)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filter.Type)
	}

	var qb strings.Builder
	qb.WriteString(`SELECT id, type, target_name, status, created_at, started_at, completed_at, result_json, error, manual FROM jobs`)
	if len(conditions) > 0 {
		qb.WriteString(" WHERE ")
		qb.WriteString(strings.Join(conditions, " AND "))
	}
	qb.WriteString(" ORDER BY created_at DESC")

	limit := filter.Limit
	if limit <= 0 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	qb.WriteString(" LIMIT ? OFFSET ?")
	query := qb.String() /* #nosec G202 -- conditions are hardcoded column names with parameterized values */
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
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

// SaveJobLogsBatch saves a batch of log entries for a job
func (s *SQLStore) SaveJobLogsBatch(ctx context.Context, jobID jobs.JobID, entries []jobs.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Start a transaction for batch insert
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			_ = err
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO job_logs (job_id, timestamp, level, message, stage) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			_ = err
		}
	}()

	for _, entry := range entries {
		if _, err := stmt.ExecContext(ctx, jobID, entry.Timestamp, entry.Level, entry.Message, entry.Stage); err != nil {
			return fmt.Errorf("failed to insert log entry: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetJobLogs retrieves log entries for a specific job
func (s *SQLStore) GetJobLogs(ctx context.Context, jobID jobs.JobID, limit int) ([]jobs.LogEntry, error) {
	if limit <= 0 {
		limit = 1000 // Default limit
	}

	query := `SELECT timestamp, level, message, stage FROM job_logs WHERE job_id = ? ORDER BY timestamp ASC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query job logs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			_ = err
		}
	}()

	var entries []jobs.LogEntry
	for rows.Next() {
		var entry jobs.LogEntry
		var stage sql.NullString

		if err := rows.Scan(&entry.Timestamp, &entry.Level, &entry.Message, &stage); err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}

		if stage.Valid {
			entry.Stage = stage.String
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}

// DB returns the underlying database connection for use by other services
func (s *SQLStore) DB() *sql.DB {
	return s.db
}
