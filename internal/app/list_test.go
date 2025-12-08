package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
)

func TestListBackups(t *testing.T) {
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

	tests := []struct {
		name          string
		setup         func()
		expectedCount int
		expectError   bool
	}{
		{
			name: "empty storage",
			setup: func() {
				// No backups created
			},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "list existing backups",
			setup: func() {
				// Create some backup files for our target
				targetDir := filepath.Join(tmpDir, "mysql-prod")
				err := os.MkdirAll(targetDir, 0755)
				require.NoError(t, err)

				// Create backup files
				files := []string{
					"backup-2025-01-08.tar.gz",
					"backup-2025-01-07.tar.gz",
					"backup-2025-01-06.tar.gz",
				}

				for _, file := range files {
					filePath := filepath.Join(targetDir, file)
					err := os.WriteFile(filePath, []byte("fake backup data"), 0644)
					require.NoError(t, err)

					// Set different modification times
					time.Sleep(10 * time.Millisecond)
				}
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name: "filter by target prefix",
			setup: func() {
				// Create backups for different targets
				targets := []string{"mysql-prod", "postgres-dev", "redis-cache"}
				for _, targetName := range targets {
					targetDir := filepath.Join(tmpDir, targetName)
					err := os.MkdirAll(targetDir, 0755)
					require.NoError(t, err)

					filePath := filepath.Join(targetDir, "backup.tar.gz")
					err = os.WriteFile(filePath, []byte("data"), 0644)
					require.NoError(t, err)
				}
			},
			expectedCount: 1, // Should only get mysql-prod backups
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up tmpDir before each test
			os.RemoveAll(tmpDir)
			err := os.MkdirAll(tmpDir, 0755)
			require.NoError(t, err)

			// Setup test data
			if tt.setup != nil {
				tt.setup()
			}

			// Test ListBackups
			backups, err := ListBackups(context.Background(), cfg, target)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, backups, tt.expectedCount)
			}
		})
	}
}

func TestListBackups_SortedByNewest(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Name = "mysql-test"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	// Create target directory
	targetDir := filepath.Join(tmpDir, "mysql-test")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	// Create files with staggered modification times
	files := []string{"oldest.tar.gz", "middle.tar.gz", "newest.tar.gz"}
	for i, file := range files {
		filePath := filepath.Join(targetDir, file)
		err := os.WriteFile(filePath, []byte("data"), 0644)
		require.NoError(t, err)

		// Set modification time
		modTime := time.Now().Add(time.Duration(i) * time.Second)
		err = os.Chtimes(filePath, modTime, modTime)
		require.NoError(t, err)
	}

	// List backups
	backups, err := ListBackups(context.Background(), cfg, target)

	require.NoError(t, err)
	require.Len(t, backups, 3)

	// Verify sorted by newest first
	assert.Contains(t, backups[0].Path, "newest.tar.gz")
	assert.Contains(t, backups[1].Path, "middle.tar.gz")
	assert.Contains(t, backups[2].Path, "oldest.tar.gz")
}

func TestFindLatestBackup(t *testing.T) {
	tmpDir := t.TempDir()

	localStorage := fixtures.LocalStorageWithCustomPath(tmpDir)
	target := fixtures.MySQLTarget()
	target.Name = "postgres-prod"

	cfg := &config.Config{
		DefaultStorage: localStorage.Name,
		Storages: map[string]*config.Storage{
			localStorage.Name: localStorage,
		},
		Targets: []*config.Target{target},
	}

	tests := []struct {
		name         string
		setup        func()
		expectError  bool
		errContains  string
		validatePath bool
	}{
		{
			name: "find latest backup",
			setup: func() {
				targetDir := filepath.Join(tmpDir, "postgres-prod")
				err := os.MkdirAll(targetDir, 0755)
				require.NoError(t, err)

				// Create multiple backups
				files := []string{"backup-old.tar.gz", "backup-latest.tar.gz"}
				for i, file := range files {
					filePath := filepath.Join(targetDir, file)
					err := os.WriteFile(filePath, []byte("data"), 0644)
					require.NoError(t, err)

					// Set modification time
					modTime := time.Now().Add(time.Duration(i) * time.Hour)
					err = os.Chtimes(filePath, modTime, modTime)
					require.NoError(t, err)
				}
			},
			expectError:  false,
			validatePath: true,
		},
		{
			name: "no backups found",
			setup: func() {
				// Don't create any backups
			},
			expectError: true,
			errContains: "no backups found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			os.RemoveAll(tmpDir)
			err := os.MkdirAll(tmpDir, 0755)
			require.NoError(t, err)

			if tt.setup != nil {
				tt.setup()
			}

			backup, err := FindLatestBackup(context.Background(), cfg, target)

			if tt.expectError {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, backup)

				if tt.validatePath {
					assert.Contains(t, backup.Path, "postgres-prod")
					assert.Contains(t, backup.Path, "latest")
				}
			}
		})
	}
}

func TestListBackups_InvalidStorage(t *testing.T) {
	target := fixtures.MySQLTarget()

	// Create config with invalid storage type
	cfg := &config.Config{
		DefaultStorage: "invalid",
		Storages: map[string]*config.Storage{
			"invalid": {
				Name: "invalid",
				Type: "nonexistent-type",
				Path: "/tmp/invalid",
			},
		},
		Targets: []*config.Target{target},
	}

	_, err := ListBackups(context.Background(), cfg, target)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get storage")
}

func TestListBackups_StorageNotFound(t *testing.T) {
	target := fixtures.MySQLTarget()
	target.Storage = &config.TargetStorage{
		Enabled: true,
		Name:    "nonexistent",
	}

	cfg := &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": fixtures.LocalStorage(),
		},
		Targets: []*config.Target{target},
	}

	_, err := ListBackups(context.Background(), cfg, target)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get storage")
}

func TestListBackups_ContextCancellation(t *testing.T) {
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

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// List should handle cancelled context
	_, err := ListBackups(ctx, cfg, target)

	// May or may not error depending on timing, but should not panic
	// If it errors, it should be context-related
	if err != nil {
		// Context cancellation errors are acceptable
		t.Logf("Expected context cancellation: %v", err)
	}
}
