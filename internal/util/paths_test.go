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
				assert.Contains(t, path, "mysql_prod/mysql/")
				assert.Contains(t, path, "/myapp_db.sql.tar.gz")
				// Path should contain timestamp in format: 2025-12-02T15-04-05Z
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
				assert.Contains(t, path, "pg_server/postgres/")
				assert.Contains(t, path, "/production.sql.tar.gz")
			},
		},
		{
			name:      "redis backup path",
			target:    "cache",
			dbType:    "redis",
			database:  "redis",
			extension: ".rdb.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "cache/redis/")
				assert.Contains(t, path, "/redis.rdb.tar.gz")
			},
		},
		{
			name:      "special characters in target name",
			target:    "my-app_v2",
			dbType:    "mysql",
			database:  "db_name",
			extension: ".sql.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "my-app_v2/mysql/")
				// Should preserve hyphens and underscores
				assert.Contains(t, path, "my-app_v2")
			},
		},
		{
			name:      "long database name",
			target:    "target",
			dbType:    "postgres",
			database:  "very_long_database_name_with_many_characters",
			extension: ".sql.tar.gz",
			validatePath: func(t *testing.T, path string) {
				assert.Contains(t, path, "very_long_database_name_with_many_characters.sql.tar.gz")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := GenerateBackupPath(tt.target, tt.dbType, tt.database, tt.extension)

			// Basic validation
			assert.NotEmpty(t, path, "path should not be empty")
			assert.True(t, strings.HasSuffix(path, tt.extension), "path should end with extension")

			// Validate path format: target/dbtype/timestamp/database.extension
			parts := strings.Split(path, "/")
			assert.GreaterOrEqual(t, len(parts), 4, "path should have at least 4 parts")
			assert.Equal(t, tt.target, parts[0], "first part should be target")
			assert.Equal(t, tt.dbType, parts[1], "second part should be dbType")

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

	// Both should follow the format test/mysql/TIMESTAMP/db.sql
	assert.Contains(t, path1, "test/mysql/")
	assert.Contains(t, path2, "test/mysql/")

	// Extract timestamps
	parts1 := strings.Split(path1, "/")
	parts2 := strings.Split(path2, "/")

	assert.Len(t, parts1, 4, "path should have 4 parts")
	assert.Len(t, parts2, 4, "path should have 4 parts")

	timestamp1 := parts1[2]
	timestamp2 := parts2[2]

	// Timestamps should be in ISO 8601-ish format (modified for filesystem)
	// Format: 2025-12-02T15-04-05Z
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z$`, timestamp1)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z$`, timestamp2)

	// Second timestamp should be later (or equal if very fast)
	assert.True(t, timestamp2 >= timestamp1, "later timestamp should be >= earlier")
}

func TestGenerateBackupPath_Consistency(t *testing.T) {
	// Same inputs at same time should produce same output
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
	assert.Equal(t, parts1[1], parts2[1], "dbType should match")
	// parts[2] is timestamp - might differ
	assert.Equal(t, parts1[3], parts2[3], "filename should match")
}

func TestGenerateBackupPath_EmptyInputs(t *testing.T) {
	// Test with empty strings - should still generate a path
	path := GenerateBackupPath("", "", "", "")
	assert.NotEmpty(t, path, "should generate path even with empty inputs")

	// With empty inputs, might generate minimal path like "//timestamp/"
	// Just verify it's not completely empty
	assert.Contains(t, path, "/", "should contain path separators")
}

func TestGenerateBackupPath_FilesystemSafe(t *testing.T) {
	// Timestamp should not contain colons (filesystem-unsafe on some systems)
	path := GenerateBackupPath("test", "mysql", "db", ".sql")

	// Extract timestamp part
	parts := strings.Split(path, "/")
	timestamp := parts[2]

	// Should not contain colons (we use hyphens instead)
	assert.NotContains(t, timestamp, ":", "timestamp should not contain colons")

	// Should contain hyphens as time separators
	assert.Contains(t, timestamp, "-", "timestamp should use hyphens")
}
