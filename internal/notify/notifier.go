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
	Target    string
	Operation string // "backup" or "restore"
	Duration  time.Duration
	Size      int64
	Path      string
	Error     error
	Timestamp time.Time
}
