package notify

import (
	"context"
	"time"
)

// Notifier sends notifications about backup/restore operations
type Notifier interface {
	// NotifySuccess sends a success notification
	NotifySuccess(ctx context.Context, msg *Message) error

	// NotifyFailure sends a failure notification
	NotifyFailure(ctx context.Context, msg *Message) error

	// Name returns the notifier name
	Name() string

	// ShouldNotifySuccess returns true if success notifications are enabled
	ShouldNotifySuccess() bool
}

// Message contains notification details
type Message struct {
	// Basic information
	Target    string
	Operation string // "backup" or "restore"
	Duration  time.Duration
	Error     error
	Timestamp time.Time

	// File path and size metrics
	Path             string
	Size             int64   // Compressed size (for backup) or backup file size (for restore)
	UncompressedSize int64   // Original size before compression
	CompressionRatio float64 // Percentage reduction

	// Storage details
	StorageName string
	StorageType string
	StoragePath string

	// Database details
	DatabaseType string
	DatabaseName string

	// Operation context
	Manual      bool
	ScheduledBy string
	DryRun      bool

	// Restore-specific
	Validations       []string
	ValidationsPassed int

	// Stage summary
	Stages []StageInfo
}

// StageInfo contains summary information about an operation stage
type StageInfo struct {
	Name     string
	Duration time.Duration
	Status   string // "completed" or "failed"
}
