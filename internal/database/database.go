package database

import (
	"context"
	"io"
	"time"
)

// Dumper represents a database that can be backed up
type Dumper interface {
	// Dump executes the database backup and writes to the provided writer
	// Returns metadata about the dump (size, duration, etc.)
	Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error)

	// Name returns a human-readable name for this database
	Name() string

	// Validate checks if the connection parameters are correct and required tools are available
	Validate(ctx context.Context) error
}

// Restorer represents a database that can be restored
type Restorer interface {
	// Restore reads from the provided reader and restores the database
	Restore(ctx context.Context, r io.Reader) error

	// Name returns a human-readable name for this database
	Name() string
}

// DumpMetadata contains information about a completed dump
type DumpMetadata struct {
	DatabaseName string
	DatabaseType string
	Size         int64
	Duration     time.Duration
	Timestamp    time.Time
}
