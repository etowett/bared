package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bared/internal/config"
)

// Local implements Storage for local filesystem
type Local struct {
	cfg *config.Storage
}

// NewLocal creates a new local storage backend
func NewLocal(cfg *config.Storage) *Local {
	return &Local{cfg: cfg}
}

// Name returns the storage name
func (l *Local) Name() string {
	return l.cfg.Name
}

// Validate checks if the storage path exists and is writable
func (l *Local) Validate(ctx context.Context) error {
	// Check if path exists
	info, err := os.Stat(l.cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("storage path does not exist: %s", l.cfg.Path)
		}
		return fmt.Errorf("failed to access storage path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("storage path is not a directory: %s", l.cfg.Path)
	}

	// Check if writable by creating a temp file
	testFile := filepath.Join(l.cfg.Path, ".bared-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("storage path is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// Store writes data from reader to local filesystem
func (l *Local) Store(ctx context.Context, path string, r io.Reader, size int64) error {
	fullPath := filepath.Join(l.cfg.Path, path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Copy data from reader to file
	_, err = io.Copy(f, r)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// Retrieve reads data from local filesystem into writer
func (l *Local) Retrieve(ctx context.Context, path string, w io.Writer) error {
	fullPath := filepath.Join(l.cfg.Path, path)

	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	return nil
}

// List returns all backup files in the storage
func (l *Local) List(ctx context.Context) ([]*BackupInfo, error) {
	var backups []*BackupInfo

	err := filepath.Walk(l.cfg.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(l.cfg.Path, path)
		if err != nil {
			return err
		}

		backups = append(backups, &BackupInfo{
			Path:         relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			StorageName:  l.cfg.Name,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	return backups, nil
}

// Delete removes a backup from local filesystem
func (l *Local) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(l.cfg.Path, path)

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
