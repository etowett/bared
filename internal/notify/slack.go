package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bared/internal/config"
)

// Slack implements Notifier for Slack webhooks
type Slack struct {
	cfg *config.Notifier
}

// NewSlack creates a new Slack notifier
func NewSlack(cfg *config.Notifier) *Slack {
	return &Slack{cfg: cfg}
}

// Name returns the notifier name
func (s *Slack) Name() string {
	return "slack"
}

// ShouldNotifySuccess returns true if success notifications are enabled
func (s *Slack) ShouldNotifySuccess() bool {
	return s.cfg.OnSuccess
}

// NotifySuccess sends a success notification to Slack
func (s *Slack) NotifySuccess(ctx context.Context, msg *Message) error {
	if !s.cfg.OnSuccess {
		return nil // Success notifications disabled
	}

	// Build operation title
	opTitle := "Backup"
	if msg.Operation == "restore" {
		opTitle = "Restore"
	}
	if msg.DryRun {
		opTitle += " (DRY-RUN)"
	}

	// Build message text
	text := fmt.Sprintf("✓ *%s Successful*\n", opTitle)
	text += fmt.Sprintf("Target: `%s`\n", msg.Target)
	text += fmt.Sprintf("Duration: %v\n", msg.Duration)

	// Add trigger information
	if msg.Manual {
		text += "Trigger: Manual\n"
	} else if msg.ScheduledBy != "" {
		text += fmt.Sprintf("Trigger: Scheduled (%s)\n", msg.ScheduledBy)
	}

	// Add size metrics for backups
	if msg.Operation == "backup" && msg.Size > 0 {
		text += "\n"
		if msg.UncompressedSize > 0 {
			text += fmt.Sprintf("Size: %s (uncompressed) → %s (compressed)\n",
				formatBytes(msg.UncompressedSize),
				formatBytes(msg.Size))
			if msg.CompressionRatio > 0 {
				text += fmt.Sprintf("Compression: %.1f%% reduction\n", msg.CompressionRatio)
			}
		} else {
			text += fmt.Sprintf("Size: %s\n", formatBytes(msg.Size))
		}
	} else if msg.Operation == "restore" && msg.Size > 0 {
		text += fmt.Sprintf("\nBackup Size: %s\n", formatBytes(msg.Size))
	}

	// Add storage details
	if msg.StorageName != "" {
		text += "\n"
		text += fmt.Sprintf("Storage: %s (%s)\n", msg.StorageName, msg.StorageType)
		if msg.Path != "" {
			text += fmt.Sprintf("Path: `%s`\n", msg.Path)
		}
	}

	// Add database details
	if msg.DatabaseName != "" {
		text += "\n"
		text += fmt.Sprintf("Database: %s (%s)\n", msg.DatabaseName, msg.DatabaseType)
	}

	// Add restore-specific validations
	if msg.Operation == "restore" && len(msg.Validations) > 0 {
		text += fmt.Sprintf("\nValidations Passed: %d\n", msg.ValidationsPassed)
	}

	// Add timestamp
	text += fmt.Sprintf("\nTime: %s\n", msg.Timestamp.Format("2006-01-02 15:04:05"))

	// Add stage summary
	if len(msg.Stages) > 0 {
		text += "\n*Stages:*\n"
		for _, stage := range msg.Stages {
			icon := "•"
			if stage.Status == "failed" {
				icon = "✗"
			}
			text += fmt.Sprintf("  %s %s: %v\n", icon, stage.Name, stage.Duration)
		}
	}

	return s.send(ctx, text, "good")
}

// NotifyFailure sends a failure notification to Slack
func (s *Slack) NotifyFailure(ctx context.Context, msg *Message) error {
	// Build operation title
	opTitle := "Backup"
	if msg.Operation == "restore" {
		opTitle = "Restore"
	}

	// Build error message
	text := fmt.Sprintf("✗ *%s Failed*\n", opTitle)
	text += fmt.Sprintf("Target: `%s`\n", msg.Target)
	text += fmt.Sprintf("Error: %v\n", msg.Error)

	// Add trigger information
	if msg.Manual {
		text += "Trigger: Manual\n"
	} else if msg.ScheduledBy != "" {
		text += fmt.Sprintf("Trigger: Scheduled (%s)\n", msg.ScheduledBy)
	}

	// Add database details
	if msg.DatabaseName != "" {
		text += fmt.Sprintf("\nDatabase: %s (%s)\n", msg.DatabaseName, msg.DatabaseType)
	}

	// Add storage details
	if msg.StorageName != "" {
		text += fmt.Sprintf("Storage: %s (%s)\n", msg.StorageName, msg.StorageType)
	}

	// Add duration if available
	if msg.Duration > 0 {
		text += fmt.Sprintf("\nDuration: %v\n", msg.Duration)
	}

	// Add timestamp
	text += fmt.Sprintf("Time: %s\n", msg.Timestamp.Format("2006-01-02 15:04:05"))

	// Add stage summary if available
	if len(msg.Stages) > 0 {
		text += "\n*Stages:*\n"
		for _, stage := range msg.Stages {
			icon := "✓"
			switch stage.Status {
			case "failed":
				icon = "✗"
			case "running":
				icon = "⋯"
			}
			text += fmt.Sprintf("  %s %s: %v\n", icon, stage.Name, stage.Duration)
		}
	}

	return s.send(ctx, text, "danger")
}

// send sends a message to Slack webhook
func (s *Slack) send(ctx context.Context, text, color string) error {
	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"text":  text,
				"color": color,
			},
		},
	}

	// Slack incoming webhooks can optionally accept a channel override.
	// Whether this is honored depends on Slack workspace / app settings.
	if ch := s.cfg.Channel; ch != "" {
		payload["channel"] = ch
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer func() {
		//nolint:errcheck // Error closing response body during cleanup is not critical
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
