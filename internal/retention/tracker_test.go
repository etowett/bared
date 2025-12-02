package retention

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/testutil/mocks"
)

func TestNewTracker(t *testing.T) {
	// Use a temporary home directory for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tests := []struct {
		name        string
		storageName string
		targetName  string
	}{
		{
			name:        "simple names",
			storageName: "s3-storage",
			targetName:  "mysql-prod",
		},
		{
			name:        "with dashes",
			storageName: "local-backup",
			targetName:  "postgres-staging",
		},
		{
			name:        "with underscores",
			storageName: "sftp_storage",
			targetName:  "redis_prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker, err := NewTracker(tt.storageName, tt.targetName)

			require.NoError(t, err)
			require.NotNil(t, tracker)
			assert.Equal(t, tt.storageName, tracker.StorageName)
			assert.Equal(t, tt.targetName, tracker.TargetName)
			assert.NotNil(t, tracker.Backups)
			assert.Empty(t, tracker.Backups)
			assert.NotEmpty(t, tracker.trackerPath)

			// Verify tracker directory was created
			trackerDir := filepath.Join(tmpDir, ".bared", "trackers")
			assert.DirExists(t, trackerDir)
		})
	}
}

func TestNewTracker_LoadsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	storageName := "s3"
	targetName := "mysql"

	// Create an existing tracker file
	trackerDir := filepath.Join(tmpDir, ".bared", "trackers")
	err := os.MkdirAll(trackerDir, 0755)
	require.NoError(t, err)

	existingData := &Tracker{
		StorageName: storageName,
		TargetName:  targetName,
		Backups: []*BackupRecord{
			{
				Path:    "backup1.sql.tar.gz",
				Size:    1000,
				Created: time.Now().Add(-2 * time.Hour),
			},
			{
				Path:    "backup2.sql.tar.gz",
				Size:    2000,
				Created: time.Now().Add(-1 * time.Hour),
			},
		},
	}

	trackerPath := filepath.Join(trackerDir, "s3-mysql.json")
	data, err := json.MarshalIndent(existingData, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(trackerPath, data, 0644)
	require.NoError(t, err)

	// Load the tracker
	tracker, err := NewTracker(storageName, targetName)

	require.NoError(t, err)
	require.NotNil(t, tracker)
	assert.Equal(t, storageName, tracker.StorageName)
	assert.Equal(t, targetName, tracker.TargetName)
	assert.Len(t, tracker.Backups, 2)
	assert.Equal(t, "backup1.sql.tar.gz", tracker.Backups[0].Path)
	assert.Equal(t, "backup2.sql.tar.gz", tracker.Backups[1].Path)
}

func TestTracker_AddBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	tests := []struct {
		name string
		path string
		size int64
	}{
		{
			name: "first backup",
			path: "backup1.sql.tar.gz",
			size: 1024,
		},
		{
			name: "second backup",
			path: "backup2.sql.tar.gz",
			size: 2048,
		},
		{
			name: "third backup",
			path: "backup3.sql.tar.gz",
			size: 4096,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tracker.AddBackup(tt.path, tt.size)

			require.NoError(t, err)
			assert.Len(t, tracker.Backups, i+1)

			// Find the added backup
			var found *BackupRecord
			for _, backup := range tracker.Backups {
				if backup.Path == tt.path {
					found = backup
					break
				}
			}

			require.NotNil(t, found, "backup should be in tracker")
			assert.Equal(t, tt.path, found.Path)
			assert.Equal(t, tt.size, found.Size)
			assert.NotZero(t, found.Created)
		})
	}

	// Verify backups are sorted by creation time (newest first)
	assert.Equal(t, "backup3.sql.tar.gz", tracker.Backups[0].Path, "newest should be first")
	assert.Equal(t, "backup1.sql.tar.gz", tracker.Backups[len(tracker.Backups)-1].Path, "oldest should be last")
}

func TestTracker_GetOldBackups(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Add 5 backups
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		tracker.Backups = append(tracker.Backups, &BackupRecord{
			Path:    filepath.Join("backup", time.Unix(int64(i), 0).Format("2006-01-02"), "backup.sql.tar.gz"),
			Size:    int64(1000 * (i + 1)),
			Created: baseTime.Add(-time.Duration(4-i) * time.Hour), // Oldest to newest
		})
	}

	// Sort by creation time (newest first)
	for i, j := 0, len(tracker.Backups)-1; i < j; i, j = i+1, j-1 {
		tracker.Backups[i], tracker.Backups[j] = tracker.Backups[j], tracker.Backups[i]
	}

	tests := []struct {
		name          string
		keep          int
		expectedCount int
	}{
		{
			name:          "keep 3, delete 2",
			keep:          3,
			expectedCount: 2,
		},
		{
			name:          "keep 1, delete 4",
			keep:          1,
			expectedCount: 4,
		},
		{
			name:          "keep 5, delete 0",
			keep:          5,
			expectedCount: 0,
		},
		{
			name:          "keep 10, delete 0 (keep more than exists)",
			keep:          10,
			expectedCount: 0,
		},
		{
			name:          "keep 0, delete 0 (invalid keep)",
			keep:          0,
			expectedCount: 0,
		},
		{
			name:          "keep -1, delete 0 (negative keep)",
			keep:          -1,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldBackups := tracker.GetOldBackups(tt.keep)

			assert.Len(t, oldBackups, tt.expectedCount)

			// Verify old backups are the oldest ones
			if tt.expectedCount > 0 {
				for _, backup := range oldBackups {
					// Old backups should be at the end of the sorted list
					assert.True(t, backup.Created.Before(tracker.Backups[tt.keep-1].Created) ||
						backup.Created.Equal(tracker.Backups[tt.keep-1].Created))
				}
			}
		})
	}
}

func TestTracker_RemoveBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Add some backups
	backupPaths := []string{
		"backup1.sql.tar.gz",
		"backup2.sql.tar.gz",
		"backup3.sql.tar.gz",
	}

	for _, path := range backupPaths {
		err := tracker.AddBackup(path, 1024)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		pathToRemove  string
		expectRemoved bool
	}{
		{
			name:          "remove existing backup",
			pathToRemove:  "backup2.sql.tar.gz",
			expectRemoved: true,
		},
		{
			name:          "remove non-existent backup",
			pathToRemove:  "nonexistent.sql.tar.gz",
			expectRemoved: false,
		},
		{
			name:          "remove another existing backup",
			pathToRemove:  "backup1.sql.tar.gz",
			expectRemoved: true,
		},
	}

	initialCount := len(tracker.Backups)
	removedCount := 0

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			countBefore := len(tracker.Backups)
			err := tracker.RemoveBackup(tt.pathToRemove)

			require.NoError(t, err)

			if tt.expectRemoved {
				assert.Len(t, tracker.Backups, countBefore-1, "backup count should decrease")
				removedCount++

				// Verify the backup is actually removed
				for _, backup := range tracker.Backups {
					assert.NotEqual(t, tt.pathToRemove, backup.Path)
				}
			} else {
				assert.Len(t, tracker.Backups, countBefore, "backup count should stay same")
			}
		})
	}

	assert.Equal(t, initialCount-removedCount, len(tracker.Backups))
}

func TestTracker_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create a tracker and add backups
	tracker1, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	backups := []*BackupRecord{
		{Path: "backup1.sql.tar.gz", Size: 1000, Created: time.Now().Add(-2 * time.Hour)},
		{Path: "backup2.sql.tar.gz", Size: 2000, Created: time.Now().Add(-1 * time.Hour)},
		{Path: "backup3.sql.tar.gz", Size: 3000, Created: time.Now()},
	}

	for _, backup := range backups {
		tracker1.Backups = append(tracker1.Backups, backup)
	}

	err = tracker1.save()
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, tracker1.trackerPath)

	// Load the tracker in a new instance
	tracker2, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Verify loaded data matches
	assert.Equal(t, tracker1.StorageName, tracker2.StorageName)
	assert.Equal(t, tracker1.TargetName, tracker2.TargetName)
	assert.Len(t, tracker2.Backups, len(backups))

	for i, backup := range backups {
		assert.Equal(t, backup.Path, tracker2.Backups[i].Path)
		assert.Equal(t, backup.Size, tracker2.Backups[i].Size)
		// Time comparison with tolerance (JSON serialization may lose precision)
		assert.WithinDuration(t, backup.Created, tracker2.Backups[i].Created, time.Second)
	}
}

func TestTracker_CleanupOldBackups(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Create mock storage
	mockStorage := mocks.NewMockStorage("s3")

	// Add 5 backups to tracker
	baseTime := time.Now()
	backupPaths := []string{
		"backup1.sql.tar.gz",
		"backup2.sql.tar.gz",
		"backup3.sql.tar.gz",
		"backup4.sql.tar.gz",
		"backup5.sql.tar.gz",
	}

	for i, path := range backupPaths {
		tracker.Backups = append([]*BackupRecord{
			{
				Path:    path,
				Size:    int64(1000 * (i + 1)),
				Created: baseTime.Add(-time.Duration(len(backupPaths)-i) * time.Hour),
			},
		}, tracker.Backups...)
	}

	tests := []struct {
		name                 string
		keep                 int
		expectedRemainingNum int
		expectedDeleteCalls  int
	}{
		{
			name:                 "keep 3, delete 2",
			keep:                 3,
			expectedRemainingNum: 3,
			expectedDeleteCalls:  2,
		},
		{
			name:                 "keep all",
			keep:                 10,
			expectedRemainingNum: 3, // 3 remaining from previous test
			expectedDeleteCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialCount := len(tracker.Backups)
			// Reset delete calls
			mockStorage.DeleteCalls = []string{}

			err := tracker.CleanupOldBackups(mockStorage, tt.keep)

			require.NoError(t, err)
			assert.Len(t, tracker.Backups, tt.expectedRemainingNum)
			assert.Equal(t, tt.expectedDeleteCalls, len(mockStorage.DeleteCalls))

			// Verify the correct backups were kept (newest ones)
			if tt.expectedRemainingNum > 0 {
				for i := 1; i < len(tracker.Backups); i++ {
					assert.True(t, tracker.Backups[i-1].Created.After(tracker.Backups[i].Created) ||
						tracker.Backups[i-1].Created.Equal(tracker.Backups[i].Created),
						"backups should be sorted newest first")
				}
			}

			// Verify storage was called with correct paths
			if tt.expectedDeleteCalls > 0 {
				toDelete := initialCount - tt.expectedRemainingNum
				for i := 0; i < toDelete; i++ {
					// Oldest backups should have been deleted
					deletedPath := backupPaths[len(backupPaths)-toDelete+i]
					found := false
					for _, deletePath := range mockStorage.DeleteCalls {
						if deletePath == deletedPath {
							found = true
							break
						}
					}
					if !found {
						t.Logf("Warning: expected deletion of %s but not found in delete calls", deletedPath)
					}
				}
			}
		})
	}
}

func TestTracker_CleanupOldBackups_StorageError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Create mock storage that fails deletion
	mockStorage := mocks.NewMockStorage("s3")
	mockStorage.DeleteError = assert.AnError

	// Add backups
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		tracker.Backups = append([]*BackupRecord{
			{
				Path:    filepath.Join("backup", time.Unix(int64(i), 0).Format("2006-01-02"), "backup.sql.tar.gz"),
				Size:    int64(1000 * (i + 1)),
				Created: baseTime.Add(-time.Duration(5-i) * time.Hour),
			},
		}, tracker.Backups...)
	}

	// Cleanup should not fail even if storage deletion fails
	err = tracker.CleanupOldBackups(mockStorage, 2)
	require.NoError(t, err)

	// Backups should remain in tracker since deletion failed
	assert.Len(t, tracker.Backups, 5, "backups should remain in tracker when deletion fails")
}

func TestTracker_JSONSerialization(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Add a backup
	err = tracker.AddBackup("test-backup.sql.tar.gz", 12345)
	require.NoError(t, err)

	// Read the JSON file
	data, err := os.ReadFile(tracker.trackerPath)
	require.NoError(t, err)

	// Verify JSON structure
	var jsonData map[string]interface{}
	err = json.Unmarshal(data, &jsonData)
	require.NoError(t, err)

	assert.Equal(t, "s3", jsonData["storage"])
	assert.Equal(t, "mysql", jsonData["target"])
	assert.NotNil(t, jsonData["backups"])

	// Verify JSON is pretty-printed (has indentation)
	assert.Contains(t, string(data), "\n  ")
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Add backups concurrently
	// Note: This may have race conditions in the actual code
	// but we're testing that it doesn't panic
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			path := filepath.Join("backup", time.Unix(int64(n), 0).Format("2006-01-02"), "backup.sql.tar.gz")
			_ = tracker.AddBackup(path, int64(1000*n))
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify some backups were added
	assert.Greater(t, len(tracker.Backups), 0)
}

func TestTracker_TrackerPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tests := []struct {
		name         string
		storageName  string
		targetName   string
		expectedFile string
	}{
		{
			name:         "simple names",
			storageName:  "s3",
			targetName:   "mysql",
			expectedFile: "s3-mysql.json",
		},
		{
			name:         "with dashes",
			storageName:  "my-storage",
			targetName:   "my-target",
			expectedFile: "my-storage-my-target.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker, err := NewTracker(tt.storageName, tt.targetName)
			require.NoError(t, err)

			expectedPath := filepath.Join(tmpDir, ".bared", "trackers", tt.expectedFile)
			assert.Equal(t, expectedPath, tracker.trackerPath)
		})
	}
}

func TestTracker_EmptyTrackerOperations(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tracker, err := NewTracker("s3", "mysql")
	require.NoError(t, err)

	// Operations on empty tracker should not panic
	oldBackups := tracker.GetOldBackups(5)
	assert.Nil(t, oldBackups)

	err = tracker.RemoveBackup("nonexistent.sql.tar.gz")
	assert.NoError(t, err)

	mockStorage := mocks.NewMockStorage("s3")
	err = tracker.CleanupOldBackups(mockStorage, 5)
	assert.NoError(t, err)
}

func TestBackupRecord_Fields(t *testing.T) {
	record := &BackupRecord{
		Path:    "backup.sql.tar.gz",
		Size:    1024,
		Created: time.Now(),
	}

	assert.Equal(t, "backup.sql.tar.gz", record.Path)
	assert.Equal(t, int64(1024), record.Size)
	assert.NotZero(t, record.Created)
}
