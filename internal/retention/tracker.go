// Package retention provides backup retention policy management and tracking.
package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"bared/internal/storage"
)

// BackupRecord represents a backup in the tracker
type BackupRecord struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
}

// Tracker manages backup metadata for retention
type Tracker struct {
	StorageName string          `json:"storage"`
	TargetName  string          `json:"target"`
	Backups     []*BackupRecord `json:"backups"`
	trackerPath string
	mu          sync.Mutex
}

// NewTracker creates a new tracker or loads existing one
func NewTracker(storageName, targetName string) (*Tracker, error) {
	tracker := &Tracker{
		StorageName: storageName,
		TargetName:  targetName,
		Backups:     []*BackupRecord{},
	}

	// Determine tracker file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	trackerDir := filepath.Join(homeDir, ".bared", "trackers")
	if err := os.MkdirAll(trackerDir, 0750); err != nil { // #nosec G301 - reduced from 0755 for security
		return nil, fmt.Errorf("failed to create tracker directory: %w", err)
	}

	tracker.trackerPath = filepath.Join(trackerDir, fmt.Sprintf("%s-%s.json", storageName, targetName))

	// Load existing tracker if it exists
	if err := tracker.load(); err != nil {
		// If file doesn't exist, that's okay
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return tracker, nil
}

// AddBackup adds a new backup to the tracker
func (t *Tracker) AddBackup(path string, size int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record := &BackupRecord{
		Path:    path,
		Size:    size,
		Created: time.Now(),
	}

	t.Backups = append(t.Backups, record)

	// Sort by creation time (newest first)
	sort.Slice(t.Backups, func(i, j int) bool {
		return t.Backups[i].Created.After(t.Backups[j].Created)
	})

	return t.save()
}

// GetOldBackups returns backups that should be deleted based on keep count
func (t *Tracker) GetOldBackups(keep int) []*BackupRecord {
	t.mu.Lock()
	defer t.mu.Unlock()

	if keep <= 0 || len(t.Backups) <= keep {
		return nil
	}

	// Return backups beyond the keep count
	return t.Backups[keep:]
}

// RemoveBackup removes a backup from the tracker
func (t *Tracker) RemoveBackup(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, backup := range t.Backups {
		if backup.Path == path {
			t.Backups = append(t.Backups[:i], t.Backups[i+1:]...)
			return t.save()
		}
	}
	return nil
}

// load loads the tracker from disk
func (t *Tracker) load() error {
	data, err := os.ReadFile(t.trackerPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, t)
}

// save saves the tracker to disk
func (t *Tracker) save() error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracker: %w", err)
	}

	if err := os.WriteFile(t.trackerPath, data, 0600); err != nil { // #nosec G306 - reduced from 0644 for security
		return fmt.Errorf("failed to write tracker: %w", err)
	}

	return nil
}

// CleanupOldBackups deletes old backups from storage and tracker
func (t *Tracker) CleanupOldBackups(stor storage.Storage, keep int) error {
	oldBackups := t.GetOldBackups(keep)
	if len(oldBackups) == 0 {
		return nil
	}

	// Delete old backups from storage
	for _, backup := range oldBackups {
		if err := stor.Delete(context.Background(), backup.Path); err != nil {
			// Log error but continue with other deletions
			fmt.Printf("Warning: failed to delete backup %s: %v\n", backup.Path, err)
		} else {
			// Remove from tracker
			if err := t.RemoveBackup(backup.Path); err != nil {
				fmt.Printf("Warning: failed to remove backup from tracker %s: %v\n", backup.Path, err)
			}
		}
	}

	return nil
}
