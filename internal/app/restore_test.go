package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
)

func TestRestoreTargetWithOptions_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with local storage
	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Name = "mysql-prod"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create a fake backup file
	targetDir := filepath.Join(tmpDir, "mysql-prod")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "mysql-prod/backup-2025-01-08.sql.tar.gz"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("fake compressed backup"), 0644)
	require.NoError(t, err)

	// Test dry-run
	options := &RestoreOptions{
		DryRun:         true,
		SkipValidation: true, // Skip database validation for tests
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.DryRun)
	assert.Equal(t, target.Name, result.Target)
	assert.Equal(t, backupPath, result.BackupPath)
	assert.NotZero(t, result.Duration)
	assert.NotEmpty(t, result.Validations)
	assert.Contains(t, result.Validations, "Database restorer created")
	assert.Contains(t, result.Validations, "Backup requires decompression")
}

func TestRestoreTargetWithOptions_SkipValidation(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Name = "postgres-dev"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create backup file
	targetDir := filepath.Join(tmpDir, "postgres-dev")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "postgres-dev/backup.sql"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("fake backup"), 0644)
	require.NoError(t, err)

	// Test with skip validation (and dry-run to avoid actual restore)
	options := &RestoreOptions{
		SkipValidation: true,
		DryRun:         true,
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)

	// Should not have "Database connection validated" in validations
	for _, v := range result.Validations {
		assert.NotContains(t, v, "Database connection validated")
	}
}

func TestRestoreTargetWithOptions_SkipBackupVerify(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Don't create backup file - testing skip backup verify
	backupPath := "mysql_test/nonexistent-backup.sql"

	options := &RestoreOptions{
		SkipBackupVerify: true,
		DryRun:           true,
	}

	// Should not fail even though backup doesn't exist
	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	// Will likely fail on database connection validation, but not on backup verification
	if err != nil {
		// Error should not be about backup not found
		assert.NotContains(t, err.Error(), "backup file not found")
	} else {
		require.NotNil(t, result)
	}
}

func TestRestoreTargetWithOptions_InvalidDatabaseType(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Conn.Type = "unsupported-db-type"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	backupPath := "mysql_test/backup.sql"

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, nil, nil)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "failed to create restorer")
}

func TestRestoreTargetWithOptions_StorageNotFound(t *testing.T) {
	target := fixtures.MySQLTarget()
	target.Storage = &config.TargetStorage{
		Enabled: true,
		Name:    "nonexistent-storage",
	}

	cfg := &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": fixtures.LocalStorage(),
		},
		Targets: []*config.Target{target},
	}

	backupPath := "mysql_test/backup.sql"

	options := &RestoreOptions{
		SkipValidation: true, // Skip database validation to test storage error
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "failed to get storage")
}

func TestRestoreTargetWithOptions_BackupNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	backupPath := "mysql_test/nonexistent-backup.sql.tar.gz"

	options := &RestoreOptions{
		DryRun:         true,
		SkipValidation: true,
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "backup file not found")
}

func TestRestoreTargetWithOptions_NilOptions(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create backup file
	targetDir := filepath.Join(tmpDir, "mysql_test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "mysql_test/backup.sql"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("fake backup"), 0644)
	require.NoError(t, err)

	// Pass nil options - should initialize with defaults
	result, _ := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, nil, nil)

	// Will fail on actual restore, but should not panic on nil options
	require.NotNil(t, result)
	assert.False(t, result.DryRun) // Default should be false
}

func TestRestoreTargetWithOptions_CompressedBackup(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create compressed backup file
	targetDir := filepath.Join(tmpDir, "mysql_test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "mysql_test/backup-2025-01-08.tar.gz"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("fake compressed data"), 0644)
	require.NoError(t, err)

	options := &RestoreOptions{
		DryRun:         true,
		SkipValidation: true,
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Validations, "Backup requires decompression")
}

func TestRestoreTargetWithOptions_UncompressedBackup(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create uncompressed backup file
	targetDir := filepath.Join(tmpDir, "mysql_test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "mysql_test/backup-2025-01-08.sql"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("fake sql data"), 0644)
	require.NoError(t, err)

	options := &RestoreOptions{
		DryRun:         true,
		SkipValidation: true,
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should NOT have decompression validation
	for _, v := range result.Validations {
		assert.NotContains(t, v, "requires decompression")
	}
}

func TestRestoreTarget(t *testing.T) {
	// Test backward compatible wrapper
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create backup file
	targetDir := filepath.Join(tmpDir, "mysql_test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "mysql_test/backup.sql"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("fake backup"), 0644)
	require.NoError(t, err)

	// Call wrapper function
	result, _ := RestoreTarget(context.Background(), cfg, target, backupPath, nil)

	// Will fail on actual restore but result should exist
	require.NotNil(t, result)
	assert.Equal(t, target.Name, result.Target)
	assert.Equal(t, backupPath, result.BackupPath)
}

func TestRestoreTargetWithOptions_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	backupPath := "mysql_test/backup.sql"

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := RestoreTargetWithOptions(ctx, cfg, target, backupPath, nil, nil)

	// Should handle cancelled context gracefully
	require.NotNil(t, result)
	if err != nil {
		// Context cancellation errors are acceptable
		t.Logf("Expected context cancellation: %v", err)
	}
}

func TestRestoreTargetWithOptions_ResultStructure(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Name = "test-target"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create backup
	targetDir := filepath.Join(tmpDir, "test-target")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "test-target/backup.sql.tar.gz"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("data"), 0644)
	require.NoError(t, err)

	options := &RestoreOptions{
		DryRun:         true,
		SkipValidation: true, // Skip database connection validation
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result structure
	assert.Equal(t, "test-target", result.Target)
	assert.Equal(t, backupPath, result.BackupPath)
	assert.True(t, result.Success)
	assert.True(t, result.DryRun)
	assert.NotZero(t, result.Duration)
	assert.Equal(t, "local", result.StorageName)
	assert.NotEmpty(t, result.Validations)
	assert.Empty(t, result.Error)
}

func TestRestoreTargetWithOptions_ValidationMessages(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create backup
	targetDir := filepath.Join(tmpDir, "mysql_test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	backupPath := "mysql_test/backup.sql"
	fullPath := filepath.Join(tmpDir, backupPath)
	err = os.WriteFile(fullPath, []byte("data"), 0644)
	require.NoError(t, err)

	options := &RestoreOptions{
		DryRun:         true,
		SkipValidation: true, // Skip database connection validation
	}

	result, err := RestoreTargetWithOptions(context.Background(), cfg, target, backupPath, options, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check for expected validation messages
	expectedValidations := []string{
		"Database restorer created",
		"Storage backend connected",
		"Storage validated",
		"Backup file verified",
	}

	for _, expected := range expectedValidations {
		found := false
		for _, validation := range result.Validations {
			if contains(validation, expected) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected validation message not found: %s", expected)
	}
}

func TestRestoreOptions_Defaults(t *testing.T) {
	options := &RestoreOptions{}

	assert.False(t, options.DryRun)
	assert.False(t, options.SkipValidation)
	assert.False(t, options.SkipBackupVerify)
}

func TestRestoreResult_Fields(t *testing.T) {
	result := &RestoreResult{
		Target:      "test-target",
		Success:     true,
		Duration:    5 * time.Second,
		BackupPath:  "test/backup.sql",
		StorageName: "local",
		DryRun:      true,
		Validations: []string{"test validation"},
	}

	assert.Equal(t, "test-target", result.Target)
	assert.True(t, result.Success)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Equal(t, "test/backup.sql", result.BackupPath)
	assert.Equal(t, "local", result.StorageName)
	assert.True(t, result.DryRun)
	assert.Len(t, result.Validations, 1)
	assert.Empty(t, result.Error)
}

func TestRestoreResult_WithError(t *testing.T) {
	testErr := errors.New("restore failed")

	result := &RestoreResult{
		Target:  "test-target",
		Success: false,
		Error:   testErr.Error(),
	}

	assert.False(t, result.Success)
	assert.Equal(t, "restore failed", result.Error)
}

// Helper function
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
