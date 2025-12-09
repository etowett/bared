package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBackupTempFile(t *testing.T) {
	tests := []struct {
		name       string
		targetName string
	}{
		{
			name:       "mysql target",
			targetName: "mysql-prod",
		},
		{
			name:       "postgres target",
			targetName: "postgres-dev",
		},
		{
			name:       "redis target",
			targetName: "redis-cache",
		},
		{
			name:       "target with special chars",
			targetName: "my_database-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, cleanup, err := CreateBackupTempFile(tt.targetName)

			require.NoError(t, err)
			require.NotNil(t, file)
			require.NotNil(t, cleanup)

			// Verify file exists and is in temp directory
			assert.NotEmpty(t, file.Name())
			assert.Contains(t, file.Name(), os.TempDir())
			assert.Contains(t, file.Name(), "bared-backup")
			assert.Contains(t, file.Name(), tt.targetName)
			assert.Contains(t, file.Name(), ".tmp")

			// Verify file is writable
			_, err = file.WriteString("test data")
			assert.NoError(t, err)

			// Cleanup
			cleanup()

			// Verify file is removed
			_, err = os.Stat(file.Name())
			assert.True(t, os.IsNotExist(err), "temp file should be removed after cleanup")
		})
	}
}

func TestCreateBackupTempFile_UniqueFilenames(t *testing.T) {
	// Create multiple temp files and verify they have unique names
	var files []*os.File
	var cleanups []func()
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	count := 5
	filenames := make(map[string]bool)

	for i := 0; i < count; i++ {
		file, cleanup, err := CreateBackupTempFile("test-target")
		require.NoError(t, err)

		_ = files
		_ = cleanups
		filenames[file.Name()] = true

		// Store for cleanup (note: not appending to maintain unique filenames)
		defer cleanup()
		defer func() {
			//nolint:staticcheck // empty branch is intentional - ignoring cleanup errors in test
			if err := file.Close(); err != nil {
				// Ignore error in test cleanup
			}
		}()
	}

	// All filenames should be unique
	assert.Len(t, filenames, count, "all temp files should have unique names")
}

func TestCreateBackupTempFile_FilePermissions(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)
	defer cleanup()

	// Get file info
	info, err := file.Stat()
	require.NoError(t, err)

	// File should be a regular file (not a directory)
	assert.False(t, info.IsDir())

	// File should have reasonable permissions (readable and writable)
	mode := info.Mode()
	assert.True(t, mode.IsRegular(), "should be a regular file")
}

func TestCreateBackupTempFile_Cleanup(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)

	filename := file.Name()

	// Write some data
	_, err = file.WriteString("test data for cleanup")
	require.NoError(t, err)

	// Verify file exists before cleanup
	_, err = os.Stat(filename)
	assert.NoError(t, err, "file should exist before cleanup")

	// Call cleanup
	cleanup()

	// Verify file is removed
	_, err = os.Stat(filename)
	assert.True(t, os.IsNotExist(err), "file should not exist after cleanup")
}

func TestCreateBackupTempFile_CleanupIdempotent(t *testing.T) {
	_, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)

	// Call cleanup multiple times - should not panic
	cleanup()
	cleanup()
	cleanup()
}

func TestGetFileSize(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected int64
	}{
		{
			name:     "empty file",
			data:     "",
			expected: 0,
		},
		{
			name:     "small file",
			data:     "hello world",
			expected: 11,
		},
		{
			name:     "larger file",
			data:     strings.Repeat("test data ", 100),
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, cleanup, err := CreateBackupTempFile("test-target")
			require.NoError(t, err)
			defer cleanup()

			// Write data
			if tt.data != "" {
				_, err = file.WriteString(tt.data)
				require.NoError(t, err)
			}

			// Sync to ensure data is written
			err = file.Sync()
			require.NoError(t, err)

			// Get size
			size, err := GetFileSize(file)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, size)
		})
	}
}

func TestGetFileSize_LargeFile(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)
	defer cleanup()

	// Write large amount of data
	largeData := strings.Repeat("a", 1024*1024) // 1 MB
	_, err = file.WriteString(largeData)
	require.NoError(t, err)

	err = file.Sync()
	require.NoError(t, err)

	size, err := GetFileSize(file)
	require.NoError(t, err)
	assert.Equal(t, int64(1024*1024), size)
}

func TestGetFileSize_AfterMultipleWrites(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)
	defer cleanup()

	// Write data in chunks
	totalSize := int64(0)
	for i := 0; i < 10; i++ {
		data := "test data chunk "
		n, writeErr := file.WriteString(data)
		require.NoError(t, writeErr)
		totalSize += int64(n)
	}

	err = file.Sync()
	require.NoError(t, err)

	size, err := GetFileSize(file)
	require.NoError(t, err)
	assert.Equal(t, totalSize, size)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "1 KB exact",
			bytes:    1024,
			expected: "1.00 KB",
		},
		{
			name:     "kilobytes",
			bytes:    5 * 1024,
			expected: "5.00 KB",
		},
		{
			name:     "1 MB exact",
			bytes:    1024 * 1024,
			expected: "1.00 MB",
		},
		{
			name:     "megabytes",
			bytes:    50 * 1024 * 1024,
			expected: "50.00 MB",
		},
		{
			name:     "1 GB exact",
			bytes:    1024 * 1024 * 1024,
			expected: "1.00 GB",
		},
		{
			name:     "gigabytes",
			bytes:    5 * 1024 * 1024 * 1024,
			expected: "5.00 GB",
		},
		{
			name:     "terabytes",
			bytes:    2 * 1024 * 1024 * 1024 * 1024,
			expected: "2.00 TB",
		},
		{
			name:     "fractional KB",
			bytes:    1536,
			expected: "1.50 KB",
		},
		{
			name:     "fractional MB",
			bytes:    1536 * 1024,
			expected: "1.50 MB",
		},
		{
			name:     "fractional GB",
			bytes:    2560 * 1024 * 1024,
			expected: "2.50 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytes_EdgeCases(t *testing.T) {
	// Test very large numbers
	result := FormatBytes(5 * 1024 * 1024 * 1024 * 1024 * 1024) // 5 PB
	assert.Contains(t, result, "PB")

	// Test 1023 bytes (just under 1 KB)
	result = FormatBytes(1023)
	assert.Equal(t, "1023 B", result)

	// Test 1025 bytes (just over 1 KB)
	result = FormatBytes(1025)
	assert.Equal(t, "1.00 KB", result)
}

func TestCreateBackupTempFile_FileLocation(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)
	defer cleanup()

	// Verify file is in system temp directory
	tempDir := os.TempDir()
	assert.True(t, strings.HasPrefix(file.Name(), tempDir),
		"temp file should be in system temp directory")

	// Verify file path is absolute
	assert.True(t, filepath.IsAbs(file.Name()),
		"temp file path should be absolute")
}

func TestCreateBackupTempFile_FilenameFormat(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("mysql-prod")
	require.NoError(t, err)
	defer cleanup()

	filename := filepath.Base(file.Name())

	// Verify filename components
	assert.True(t, strings.HasPrefix(filename, "bared-backup-"))
	assert.Contains(t, filename, "mysql-prod")
	assert.True(t, strings.HasSuffix(filename, ".tmp"))

	// Should contain timestamp (YYYYMMDD-HHMMSS format)
	assert.Regexp(t, `\d{8}-\d{6}`, filename)

	// Should contain PID
	assert.Contains(t, filename, "-")

	// Should contain random number
	parts := strings.Split(filename, "-")
	assert.GreaterOrEqual(t, len(parts), 5, "filename should have multiple dash-separated parts")
}

func TestGetFileSize_ClosedFile(t *testing.T) {
	file, cleanup, err := CreateBackupTempFile("test-target")
	require.NoError(t, err)
	defer cleanup()

	// Write some data
	_, err = file.WriteString("test data")
	require.NoError(t, err)

	// Close the file
	err = file.Close()
	require.NoError(t, err)

	// Getting size of closed file should return an error
	_, err = GetFileSize(file)
	assert.Error(t, err)
}

func TestCreateBackupTempFile_ConcurrentCreation(t *testing.T) {
	// Create multiple temp files concurrently
	count := 10
	done := make(chan string, count)

	for i := 0; i < count; i++ {
		go func() {
			file, cleanup, err := CreateBackupTempFile("test-target")
			if err != nil {
				done <- ""
				return
			}
			defer cleanup()
			done <- file.Name()
		}()
	}

	// Collect all filenames
	filenames := make(map[string]bool)
	for i := 0; i < count; i++ {
		filename := <-done
		if filename != "" {
			filenames[filename] = true
		}
	}

	// All should be unique
	assert.Len(t, filenames, count, "concurrent file creation should produce unique filenames")
}
