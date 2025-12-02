package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
)

func TestNewLocal(t *testing.T) {
	cfg := &config.Storage{
		Name: "local-test",
		Type: "local",
		Path: "/tmp/test-backups",
	}

	local := NewLocal(cfg)

	assert.NotNil(t, local)
	assert.Equal(t, cfg, local.cfg)
}

func TestLocal_Name(t *testing.T) {
	cfg := &config.Storage{
		Name: "my-local-storage",
		Type: "local",
		Path: "/tmp/backups",
	}

	local := NewLocal(cfg)
	assert.Equal(t, "my-local-storage", local.Name())
}

func TestLocal_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T) string // Returns path
		wantErr     bool
		errContains string
	}{
		{
			name: "valid directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name: "non-existent directory",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name: "path is a file, not directory",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "file.txt")
				os.WriteFile(filePath, []byte("test"), 0644)
				return filePath
			},
			wantErr:     true,
			errContains: "not a directory",
		},
		{
			name: "directory not writable",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				// Make directory read-only
				os.Chmod(tmpDir, 0444)
				t.Cleanup(func() {
					os.Chmod(tmpDir, 0755) // Restore permissions for cleanup
				})
				return tmpDir
			},
			wantErr:     true,
			errContains: "not writable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			cfg := &config.Storage{
				Name: "test",
				Type: "local",
				Path: path,
			}

			local := NewLocal(cfg)
			err := local.Validate(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLocal_Store(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		data        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "store simple file",
			path:    "backup.sql",
			data:    "test backup data",
			wantErr: false,
		},
		{
			name:    "store in subdirectory",
			path:    "mysql/2025-12-02/backup.sql",
			data:    "mysql backup data",
			wantErr: false,
		},
		{
			name:    "store with deep nesting",
			path:    "target/mysql/2025-12-02T10-30-00Z/testdb.sql.tar.gz",
			data:    "nested backup",
			wantErr: false,
		},
		{
			name:    "store large data",
			path:    "large.sql",
			data:    strings.Repeat("data\n", 10000),
			wantErr: false,
		},
		{
			name:    "store empty file",
			path:    "empty.sql",
			data:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Storage{
				Name: "test",
				Type: "local",
				Path: tmpDir,
			}

			local := NewLocal(cfg)
			reader := strings.NewReader(tt.data)
			err := local.Store(context.Background(), tt.path, reader, int64(len(tt.data)))

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)

				// Verify file was created
				fullPath := filepath.Join(tmpDir, tt.path)
				assert.FileExists(t, fullPath)

				// Verify content
				content, err := os.ReadFile(fullPath)
				require.NoError(t, err)
				assert.Equal(t, tt.data, string(content))
			}
		})
	}
}

func TestLocal_Store_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Storage{
		Name: "test",
		Type: "local",
		Path: tmpDir,
	}

	local := NewLocal(cfg)
	path := "level1/level2/level3/backup.sql"
	data := "test data"

	reader := strings.NewReader(data)
	err := local.Store(context.Background(), path, reader, int64(len(data)))

	require.NoError(t, err)

	// Verify all directories were created
	fullPath := filepath.Join(tmpDir, path)
	assert.FileExists(t, fullPath)

	// Verify parent directories exist
	assert.DirExists(t, filepath.Join(tmpDir, "level1"))
	assert.DirExists(t, filepath.Join(tmpDir, "level1/level2"))
	assert.DirExists(t, filepath.Join(tmpDir, "level1/level2/level3"))
}

func TestLocal_Retrieve(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   func(tmpDir string) string // Returns path
		wantErr     bool
		errContains string
	}{
		{
			name: "retrieve existing file",
			setupFile: func(tmpDir string) string {
				path := "backup.sql"
				os.WriteFile(filepath.Join(tmpDir, path), []byte("test data"), 0644)
				return path
			},
			wantErr: false,
		},
		{
			name: "retrieve from subdirectory",
			setupFile: func(tmpDir string) string {
				path := "mysql/backup.sql"
				os.MkdirAll(filepath.Join(tmpDir, "mysql"), 0755)
				os.WriteFile(filepath.Join(tmpDir, path), []byte("mysql data"), 0644)
				return path
			},
			wantErr: false,
		},
		{
			name: "retrieve non-existent file",
			setupFile: func(tmpDir string) string {
				return "nonexistent.sql"
			},
			wantErr:     true,
			errContains: "failed to open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Storage{
				Name: "test",
				Type: "local",
				Path: tmpDir,
			}

			path := tt.setupFile(tmpDir)
			local := NewLocal(cfg)

			var buf bytes.Buffer
			err := local.Retrieve(context.Background(), path, &buf)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

func TestLocal_Retrieve_ReturnsCorrectContent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Storage{
		Name: "test",
		Type: "local",
		Path: tmpDir,
	}

	// Create a file with known content
	expectedContent := "This is the backup content\nWith multiple lines\n"
	path := "backup.sql"
	os.WriteFile(filepath.Join(tmpDir, path), []byte(expectedContent), 0644)

	local := NewLocal(cfg)
	var buf bytes.Buffer
	err := local.Retrieve(context.Background(), path, &buf)

	require.NoError(t, err)
	assert.Equal(t, expectedContent, buf.String())
}

func TestLocal_List(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  func(tmpDir string)
		wantCount   int
		validate    func(*testing.T, []*BackupInfo)
	}{
		{
			name: "list empty directory",
			setupFiles: func(tmpDir string) {
				// No files
			},
			wantCount: 0,
		},
		{
			name: "list single file",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "backup.sql"), []byte("data"), 0644)
			},
			wantCount: 1,
		},
		{
			name: "list multiple files",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "backup1.sql"), []byte("data1"), 0644)
				os.WriteFile(filepath.Join(tmpDir, "backup2.sql"), []byte("data2"), 0644)
				os.WriteFile(filepath.Join(tmpDir, "backup3.sql"), []byte("data3"), 0644)
			},
			wantCount: 3,
		},
		{
			name: "list files in subdirectories",
			setupFiles: func(tmpDir string) {
				os.MkdirAll(filepath.Join(tmpDir, "mysql"), 0755)
				os.MkdirAll(filepath.Join(tmpDir, "postgres"), 0755)
				os.WriteFile(filepath.Join(tmpDir, "mysql/backup1.sql"), []byte("mysql"), 0644)
				os.WriteFile(filepath.Join(tmpDir, "postgres/backup2.sql"), []byte("pg"), 0644)
			},
			wantCount: 2,
			validate: func(t *testing.T, backups []*BackupInfo) {
				paths := make([]string, len(backups))
				for i, b := range backups {
					paths[i] = b.Path
				}
				assert.Contains(t, paths, filepath.Join("mysql", "backup1.sql"))
				assert.Contains(t, paths, filepath.Join("postgres", "backup2.sql"))
			},
		},
		{
			name: "skip hidden files",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "backup.sql"), []byte("data"), 0644)
				os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
			},
			wantCount: 1,
		},
		{
			name: "skip directories",
			setupFiles: func(tmpDir string) {
				os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
				os.WriteFile(filepath.Join(tmpDir, "backup.sql"), []byte("data"), 0644)
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Storage{
				Name: "test-storage",
				Type: "local",
				Path: tmpDir,
			}

			tt.setupFiles(tmpDir)

			local := NewLocal(cfg)
			backups, err := local.List(context.Background())

			require.NoError(t, err)
			assert.Len(t, backups, tt.wantCount)

			// Verify all backups have required fields
			for _, backup := range backups {
				assert.NotEmpty(t, backup.Path)
				assert.Greater(t, backup.Size, int64(0))
				assert.NotZero(t, backup.LastModified)
				assert.Equal(t, "test-storage", backup.StorageName)
			}

			if tt.validate != nil {
				tt.validate(t, backups)
			}
		})
	}
}

func TestLocal_Delete(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   func(tmpDir string) string
		wantErr     bool
		errContains string
	}{
		{
			name: "delete existing file",
			setupFile: func(tmpDir string) string {
				path := "backup.sql"
				os.WriteFile(filepath.Join(tmpDir, path), []byte("data"), 0644)
				return path
			},
			wantErr: false,
		},
		{
			name: "delete file in subdirectory",
			setupFile: func(tmpDir string) string {
				path := "mysql/backup.sql"
				os.MkdirAll(filepath.Join(tmpDir, "mysql"), 0755)
				os.WriteFile(filepath.Join(tmpDir, path), []byte("data"), 0644)
				return path
			},
			wantErr: false,
		},
		{
			name: "delete non-existent file",
			setupFile: func(tmpDir string) string {
				return "nonexistent.sql"
			},
			wantErr:     true,
			errContains: "failed to delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Storage{
				Name: "test",
				Type: "local",
				Path: tmpDir,
			}

			path := tt.setupFile(tmpDir)
			local := NewLocal(cfg)

			err := local.Delete(context.Background(), path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)

				// Verify file was deleted
				fullPath := filepath.Join(tmpDir, path)
				_, err := os.Stat(fullPath)
				assert.True(t, os.IsNotExist(err))
			}
		})
	}
}

func TestLocal_StoreAndRetrieve_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Storage{
		Name: "test",
		Type: "local",
		Path: tmpDir,
	}

	local := NewLocal(cfg)
	path := "test/backup.sql"
	originalData := "This is test backup data\nWith multiple lines\nAnd special characters: !@#$%\n"

	// Store
	reader := strings.NewReader(originalData)
	err := local.Store(context.Background(), path, reader, int64(len(originalData)))
	require.NoError(t, err)

	// Retrieve
	var buf bytes.Buffer
	err = local.Retrieve(context.Background(), path, &buf)
	require.NoError(t, err)

	// Verify content matches
	assert.Equal(t, originalData, buf.String())
}

func TestLocal_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Storage{
		Name: "test",
		Type: "local",
		Path: tmpDir,
	}

	local := NewLocal(cfg)

	// Currently, local storage doesn't check context
	// but we test to ensure no panic
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// These should still work (local operations are fast)
	_ = local.Validate(ctx)
	_ = local.Store(ctx, "test.sql", strings.NewReader("data"), 4)
	_, _ = local.List(ctx)
}

func TestLocal_List_SortsbyModTime(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Storage{
		Name: "test",
		Type: "local",
		Path: tmpDir,
	}

	// Create files with different modification times
	file1 := filepath.Join(tmpDir, "backup1.sql")
	file2 := filepath.Join(tmpDir, "backup2.sql")
	file3 := filepath.Join(tmpDir, "backup3.sql")

	os.WriteFile(file1, []byte("data1"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(file2, []byte("data2"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(file3, []byte("data3"), 0644)

	local := NewLocal(cfg)
	backups, err := local.List(context.Background())

	require.NoError(t, err)
	require.Len(t, backups, 3)

	// Verify files have different mod times
	assert.NotEqual(t, backups[0].LastModified, backups[1].LastModified)
	assert.NotEqual(t, backups[1].LastModified, backups[2].LastModified)
}

func TestLocal_Store_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Storage{
		Name: "test",
		Type: "local",
		Path: tmpDir,
	}

	local := NewLocal(cfg)
	path := "backup.sql"

	// Store first version
	data1 := "original data"
	err := local.Store(context.Background(), path, strings.NewReader(data1), int64(len(data1)))
	require.NoError(t, err)

	// Store second version (overwrite)
	data2 := "updated data that is longer"
	err = local.Store(context.Background(), path, strings.NewReader(data2), int64(len(data2)))
	require.NoError(t, err)

	// Retrieve and verify we get the updated content
	var buf bytes.Buffer
	err = local.Retrieve(context.Background(), path, &buf)
	require.NoError(t, err)
	assert.Equal(t, data2, buf.String())
}
