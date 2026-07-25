package storage

import (
	"context"
	"io"
	"time"
)

// Storage represents a backup storage backend
type Storage interface {
	// Store writes data from reader to the storage backend
	// path is relative to the storage's configured base path
	Store(ctx context.Context, path string, r io.Reader, size int64) error

	// Retrieve reads data from storage into the writer
	Retrieve(ctx context.Context, path string, w io.Writer) error

	// List returns all backup files in the storage
	List(ctx context.Context) ([]*BackupInfo, error)

	// Delete removes a backup from storage
	Delete(ctx context.Context, path string) error

	// Name returns the storage backend name
	Name() string

	// Validate checks if the storage is accessible
	Validate(ctx context.Context) error

	// Exists checks if a backup file exists
	Exists(ctx context.Context, path string) (bool, error)

	// GetInfo returns metadata about a backup file
	GetInfo(ctx context.Context, path string) (*BackupInfo, error)
}

// BackupInfo contains metadata about a stored backup
type BackupInfo struct {
	Path         string
	Size         int64
	LastModified time.Time
	StorageName  string
}
