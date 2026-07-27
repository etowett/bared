package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

// Helper to create a basic config
func createTestConfig() *config.Config {
	localStorage := fixtures.LocalStorage()
	return &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{
			fixtures.MySQLTarget(),
		},
	}
}

func TestBackupTarget_StructureAndErrorHandling(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]

	// Will fail because mysqldump doesn't exist, but tests orchestration
	result, err := BackupTarget(context.Background(), cfg, target, nil)

	// Verify result structure exists (even if operation failed)
	require.NotNil(t, result)
	assert.Equal(t, target.Name, result.Target)
	assert.False(t, result.Success)
	assert.NotNil(t, result.Error)

	// Error should be descriptive
	assert.Error(t, err)
	t.Logf("Expected error (no mysqldump): %v", err)
}

func TestBackupTarget_WithCompression(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]
	target.Compress = &config.CompressionOpts{
		Enabled: true,
		Type:    "tgz",
	}

	result, _ := BackupTarget(context.Background(), cfg, target, nil)

	require.NotNil(t, result)
	assert.Contains(t, result.BackupPath, ".tar.gz", "compressed backup should have .tar.gz extension")
}

func TestBackupTarget_WithoutCompression(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]
	target.Compress = nil

	result, _ := BackupTarget(context.Background(), cfg, target, nil)

	require.NotNil(t, result)
	assert.Contains(t, result.BackupPath, ".sql", "uncompressed backup should have .sql extension")
}

func TestBackupTarget_InvalidDatabaseType(t *testing.T) {
	cfg := createTestConfig()
	cfg.Targets[0].Conn.Type = "unsupported-db"

	result, err := BackupTarget(context.Background(), cfg, cfg.Targets[0], nil)

	assert.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, err.Error(), "failed to create dumper")
	assert.False(t, result.Success)
}

func TestBackupTarget_StorageNotFound(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]
	target.Storage = &config.TargetStorage{
		Enabled: true,
		Name:    "nonexistent-storage",
	}

	result, err := BackupTarget(context.Background(), cfg, target, nil)

	assert.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, err.Error(), "failed to get storage")
}

func TestBackupTarget_InvalidCompressionType(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]
	target.Compress = &config.CompressionOpts{
		Enabled: true,
		Type:    "unsupported",
	}

	result, err := BackupTarget(context.Background(), cfg, target, nil)

	assert.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, err.Error(), "failed to create compressor")
}

func TestBackupResult_Fields(t *testing.T) {
	result := &BackupResult{
		Target:      "mysql-prod",
		Success:     true,
		Error:       "",
		Duration:    5 * time.Second,
		BackupPath:  "mysql-prod/2025-12-02/backup.sql",
		Size:        1024,
		StorageName: "s3-storage",
	}

	assert.Equal(t, "mysql-prod", result.Target)
	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Equal(t, "mysql-prod/2025-12-02/backup.sql", result.BackupPath)
	assert.Equal(t, int64(1024), result.Size)
	assert.Equal(t, "s3-storage", result.StorageName)
}

func TestBackupResult_WithError(t *testing.T) {
	testErr := fmt.Errorf("backup failed")
	result := &BackupResult{
		Target:  "mysql-prod",
		Success: false,
		Error:   testErr.Error(),
	}

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
	assert.Equal(t, "backup failed", result.Error)
}

func TestBackupTarget_DifferentDatabaseTypes(t *testing.T) {
	tests := []struct {
		name   string
		target *config.Target
	}{
		{name: "MySQL", target: fixtures.MySQLTarget()},
		{name: "PostgreSQL", target: fixtures.PostgresTarget()},
		{name: "Redis", target: fixtures.RedisTarget()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig()
			cfg.Targets = []*config.Target{tt.target}

			result, _ := BackupTarget(context.Background(), cfg, tt.target, nil)

			require.NotNil(t, result)
			assert.Equal(t, tt.target.Name, result.Target)
			assert.NotEmpty(t, result.BackupPath)
		})
	}
}

func TestBackupTarget_StorageNameRecorded(t *testing.T) {
	localStorage := fixtures.LocalStorage()
	localStorage.Name = "test-storage"

	target := fixtures.MySQLTarget()
	// Use default storage instead of target-specific storage
	target.Storage = nil

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	result, _ := BackupTarget(context.Background(), cfg, cfg.Targets[0], nil)

	require.NotNil(t, result)
	assert.Equal(t, "test-storage", result.StorageName)
}

func TestBackupTarget_ContextCancellation(t *testing.T) {
	cfg := createTestConfig()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := BackupTarget(ctx, cfg, cfg.Targets[0], nil)

	// Should handle cancelled context gracefully
	require.NotNil(t, result)
	if err != nil {
		t.Logf("Backup with cancelled context: err=%v", err)
	}
}

func TestBackupTarget_PathContainsTargetInfo(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]

	result, _ := BackupTarget(context.Background(), cfg, target, nil)

	require.NotNil(t, result)
	path := strings.ToLower(result.BackupPath)

	// Path should contain target name and start with target name
	assert.Contains(t, path, strings.ToLower(target.Name), "backup path should contain target name: %s", result.BackupPath)
	assert.True(t, strings.HasPrefix(path, strings.ToLower(target.Name)+"/"), "backup path should start with target name: %s", result.BackupPath)

	// Path should follow new format: target/backup-timestamp.ext
	assert.Contains(t, path, "/backup-", "backup path should contain /backup- prefix: %s", result.BackupPath)
}

func TestBackupTarget_DurationMeasured(t *testing.T) {
	cfg := createTestConfig()

	result, _ := BackupTarget(context.Background(), cfg, cfg.Targets[0], nil)

	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
}

func TestSendNotifications_NoNotifiers(_ *testing.T) {
	cfg := createTestConfig()
	result := &BackupResult{
		Target:     "mysql-prod",
		Success:    true,
		Duration:   5 * time.Second,
		BackupPath: "backup.sql",
	}

	// Should not panic with no notifiers
	sendNotifications(context.Background(), cfg, cfg.Targets[0], result, nil)
}

func TestBackupTarget_ExcludeTablesConfiguration(t *testing.T) {
	cfg := createTestConfig()
	target := fixtures.MySQLTargetWithExcludeTables()
	cfg.Targets = []*config.Target{target}

	result, _ := BackupTarget(context.Background(), cfg, target, nil)

	require.NotNil(t, result)
	assert.Equal(t, target.Name, result.Target)
}

func TestBackupTarget_AdditionalArgsConfiguration(t *testing.T) {
	cfg := createTestConfig()
	target := cfg.Targets[0]
	target.AdditionalArgs = []string{"--single-transaction", "--quick"}

	result, _ := BackupTarget(context.Background(), cfg, target, nil)

	require.NotNil(t, result)
	assert.Equal(t, target.Name, result.Target)
}

func TestBackupTarget_CompressionExtensions(t *testing.T) {
	tests := []struct {
		name            string
		compressionType string
		wantExtension   string
	}{
		{
			name:          "no compression",
			wantExtension: ".sql",
		},
		{
			name:            "tgz compression",
			compressionType: "tgz",
			wantExtension:   ".tar.gz",
		},
		{
			name:            "tar.gz compression",
			compressionType: "tar.gz",
			wantExtension:   ".tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig()
			target := cfg.Targets[0]

			if tt.compressionType != "" {
				target.Compress = &config.CompressionOpts{
					Enabled: true,
					Type:    tt.compressionType,
				}
			} else {
				target.Compress = nil
			}

			result, _ := BackupTarget(context.Background(), cfg, target, nil)

			require.NotNil(t, result)
			assert.Contains(t, result.BackupPath, tt.wantExtension)
		})
	}
}

func TestBackupTarget_ResultInitialization(t *testing.T) {
	cfg := createTestConfig()

	result, _ := BackupTarget(context.Background(), cfg, cfg.Targets[0], nil)

	// All fields should be initialized
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Target)
	assert.NotEmpty(t, result.BackupPath)
	// Success may be false, but result structure is complete
}

// TestE2E_BackupListRestore_NewPathFormat is an end-to-end integration test
// that validates the full backup→list→restore cycle with the new path format
func TestE2E_BackupListRestore_NewPathFormat(t *testing.T) {
	// Note: This test will fail during backup execution because mysqldump doesn't exist,
	// but it validates the path generation and discovery logic

	tmpDir := t.TempDir()

	// Create config with local storage
	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Name = "e2e-test"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Step 1: Attempt backup (will fail due to no mysqldump, but path is generated)
	result, _ := BackupTarget(context.Background(), cfg, target, nil)
	require.NotNil(t, result)

	// Verify new path format: target/backup-timestamp.extension
	t.Logf("Generated backup path: %s", result.BackupPath)
	assert.True(t, strings.Contains(result.BackupPath, "/backup-"), "path should contain /backup- prefix")
	assert.True(t, strings.Contains(result.BackupPath, "T"), "path should contain ISO timestamp marker")
	assert.True(t, strings.Contains(result.BackupPath, "Z"), "path should contain UTC marker")

	// Verify 2-part path structure
	parts := strings.Split(result.BackupPath, "/")
	assert.Equal(t, 2, len(parts), "path should have exactly 2 parts: target/filename")
	assert.Equal(t, "e2e-test", parts[0], "first part should be target name")
	assert.True(t, strings.HasPrefix(parts[1], "backup-"), "second part should start with 'backup-'")

	// Step 2: Manually create a backup file to test listing and restore
	// (Since actual backup failed, we create a mock file)
	targetDir := filepath.Join(tmpDir, "e2e-test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	mockBackupPath := "e2e-test/backup-2026-01-05T10-00-00Z.sql.tar.gz"
	mockBackupFile := filepath.Join(tmpDir, mockBackupPath)
	err = os.WriteFile(mockBackupFile, []byte("mock backup data"), 0644)
	require.NoError(t, err)

	// Step 3: List backups
	backups, err := ListBackups(context.Background(), cfg, target)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(backups), 1, "should find at least one backup")

	// Verify backup was discovered
	var found bool
	for _, backup := range backups {
		if strings.Contains(backup.Path, "backup-2026-01-05") {
			found = true
			assert.True(t, strings.HasPrefix(backup.Path, "e2e-test/"), "backup path should start with target name")
			assert.True(t, strings.Contains(backup.Path, "backup-"), "backup path should contain 'backup-' prefix")
		}
	}
	assert.True(t, found, "should find the mock backup in listing")

	// Step 4: Find latest backup
	latest, err := FindLatestBackup(context.Background(), cfg, target)
	require.NoError(t, err)
	assert.NotNil(t, latest)
	assert.True(t, strings.HasPrefix(latest.Path, "e2e-test/backup-"), "latest backup should follow new path format")

	// Step 5: Attempt restore (dry-run) - will fail due to no mysql, but validates path handling
	restoreResult, _ := RestoreTargetWithOptions(
		context.Background(), cfg, target, mockBackupPath,
		&RestoreOptions{DryRun: true, SkipValidation: true}, nil)

	// Even if restore fails, the result structure should be valid
	require.NotNil(t, restoreResult)
	t.Logf("Restore attempt completed with result: %+v", restoreResult)
}
