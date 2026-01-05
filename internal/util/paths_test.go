package util

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateBackupPath(t *testing.T) {
	tests := []struct {
		name         string
		target       string
		dbType       string
		database     string
		extension    string
		validatePath func(*testing.T, string)
	}{
		{
			name:      "mysql backup path",
			target:    "mysql_prod",
			dbType:    "mysql",
			database:  "myapp_db",
			extension: ".sql.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "mysql_prod/backup-")
				assert.Contains(t, path, ".sql.tar.gz")
				// Path should contain timestamp in format: 2026-01-05T17-29-06Z
				assert.Contains(t, path, "T")
				assert.Contains(t, path, "Z")
			},
		},
		{
			name:      "postgres backup path",
			target:    "pg_server",
			dbType:    "postgres",
			database:  "production",
			extension: ".sql.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "pg_server/backup-")
				assert.Contains(t, path, ".sql.tar.gz")
			},
		},
		{
			name:      "redis backup path",
			target:    "cache",
			dbType:    "redis",
			database:  "redis",
			extension: ".rdb.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "cache/backup-")
				assert.Contains(t, path, ".rdb.tar.gz")
			},
		},
		{
			name:      "special characters in target name",
			target:    "my-app_v2",
			dbType:    "mysql",
			database:  "db_name",
			extension: ".sql.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "my-app_v2/backup-")
				// Should preserve hyphens and underscores
				assert.Contains(t, path, "my-app_v2")
			},
		},
		{
			name:      "uncompressed backup",
			target:    "target",
			dbType:    "postgres",
			database:  "very_long_database_name_with_many_characters",
			extension: ".sql",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "backup-")
				assert.True(t, strings.HasSuffix(path, ".sql"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := GenerateBackupPath(tt.target, tt.dbType, tt.database, tt.extension)

			// Basic validation
			assert.NotEmpty(t, path, "path should not be empty")
			assert.True(t, strings.HasSuffix(path, tt.extension), "path should end with extension")

			// Validate path format: target/backup-timestamp.extension
			parts := strings.Split(path, "/")
			assert.Equal(t, 2, len(parts), "path should have exactly 2 parts")
			assert.Equal(t, tt.target, parts[0], "first part should be target")
			assert.True(t, strings.HasPrefix(parts[1], "backup-"), "second part should start with 'backup-'")

			// Custom validation
			if tt.validatePath != nil {
				tt.validatePath(t, path)
			}
		})
	}
}

func TestGenerateBackupPath_TimestampFormat(t *testing.T) {
	// Generate two paths in quick succession
	path1 := GenerateBackupPath("test", "mysql", "db", ".sql")
	time.Sleep(2 * time.Second)
	path2 := GenerateBackupPath("test", "mysql", "db", ".sql")

	// Paths should be different (different timestamps)
	assert.NotEqual(t, path1, path2, "paths generated at different times should differ")

	// Both should follow the format test/backup-TIMESTAMP.sql
	assert.Contains(t, path1, "test/backup-")
	assert.Contains(t, path2, "test/backup-")

	// Extract timestamps from filenames
	parts1 := strings.Split(path1, "/")
	parts2 := strings.Split(path2, "/")

	assert.Len(t, parts1, 2, "path should have 2 parts")
	assert.Len(t, parts2, 2, "path should have 2 parts")

	filename1 := parts1[1] // e.g., "backup-2026-01-05T17-29-06Z.sql"
	filename2 := parts2[1]

	// Extract timestamp: strip "backup-" prefix and extension
	timestamp1 := strings.TrimPrefix(filename1, "backup-")
	timestamp1 = timestamp1[:strings.LastIndex(timestamp1, ".")]

	timestamp2 := strings.TrimPrefix(filename2, "backup-")
	timestamp2 = timestamp2[:strings.LastIndex(timestamp2, ".")]

	// Timestamps should be in ISO 8601-ish format (modified for filesystem)
	// Format: 2026-01-05T17-29-06Z
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z$`, timestamp1)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z$`, timestamp2)

	// Second timestamp should be later (or equal if very fast)
	assert.True(t, timestamp2 >= timestamp1, "later timestamp should be >= earlier")
}

func TestGenerateBackupPath_Consistency(t *testing.T) {
	// Same inputs at same time should produce same output structure
	target := "test_target"
	dbType := "mysql"
	database := "testdb"
	extension := ".sql.tar.gz"

	path1 := GenerateBackupPath(target, dbType, database, extension)
	path2 := GenerateBackupPath(target, dbType, database, extension)

	// Paths might differ by timestamp if called at different times
	// But structure should be consistent
	parts1 := strings.Split(path1, "/")
	parts2 := strings.Split(path2, "/")

	assert.Equal(t, parts1[0], parts2[0], "target should match")
	assert.True(t, strings.HasPrefix(parts1[1], "backup-"), "filename should start with backup-")
	assert.True(t, strings.HasPrefix(parts2[1], "backup-"), "filename should start with backup-")
}

func TestGenerateBackupPath_EmptyInputs(t *testing.T) {
	// Test with empty strings - should still generate a path
	path := GenerateBackupPath("", "", "", "")
	assert.NotEmpty(t, path, "should generate path even with empty inputs")

	// Should still contain backup- prefix in filename
	assert.Contains(t, path, "backup-")
}

func TestGenerateBackupPath_FilesystemSafe(t *testing.T) {
	// Timestamp should not contain colons (filesystem-unsafe on some systems)
	path := GenerateBackupPath("test", "mysql", "db", ".sql")

	// Extract timestamp from filename
	parts := strings.Split(path, "/")
	filename := parts[1] // e.g., "backup-2026-01-05T17-29-06Z.sql"

	// Extract timestamp portion between "backup-" and extension
	timestampStart := len("backup-")
	timestampEnd := strings.LastIndex(filename, ".")
	timestamp := filename[timestampStart:timestampEnd]

	// Should not contain colons (we use hyphens instead)
	assert.NotContains(t, timestamp, ":", "timestamp should not contain colons")

	// Should contain hyphens as time separators
	assert.Contains(t, timestamp, "-", "timestamp should use hyphens")
}

func TestGenerateBackupPath_FilenameStructure(t *testing.T) {
	// Test that filename has the expected structure
	path := GenerateBackupPath("target", "mysql", "db", ".tar.gz")
	parts := strings.Split(path, "/")

	assert.Len(t, parts, 2)
	assert.Equal(t, "target", parts[0])

	filename := parts[1]
	assert.True(t, strings.HasPrefix(filename, "backup-"))
	assert.True(t, strings.HasSuffix(filename, ".tar.gz"))
	assert.Contains(t, filename, "T") // ISO timestamp marker
	assert.Contains(t, filename, "Z") // UTC marker
}

func TestGenerateBackupPath_UniquePaths(t *testing.T) {
	// Same target, different calls should generate unique paths due to timestamps
	path1 := GenerateBackupPath("target", "mysql", "db", ".sql")
	time.Sleep(2 * time.Second)
	path2 := GenerateBackupPath("target", "mysql", "db", ".sql")

	assert.NotEqual(t, path1, path2, "timestamps should make paths unique")
	assert.True(t, strings.HasPrefix(path1, "target/backup-"))
	assert.True(t, strings.HasPrefix(path2, "target/backup-"))
}
