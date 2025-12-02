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

	text := fmt.Sprintf("✓ *Backup Successful*\n"+
		"Target: `%s`\n"+
		"Duration: %v\n"+
		"Path: `%s`\n"+
		"Time: %s",
		msg.Target,
		msg.Duration,
		msg.Path,
		msg.Timestamp.Format("2006-01-02 15:04:05"),
	)

	return s.send(ctx, text, "good")
}

// NotifyFailure sends a failure notification to Slack
func (s *Slack) NotifyFailure(ctx context.Context, msg *Message) error {
	text := fmt.Sprintf("✗ *Backup Failed*\n"+
		"Target: `%s`\n"+
		"Error: %v\n"+
		"Time: %s",
		msg.Target,
		msg.Error,
		msg.Timestamp.Format("2006-01-02 15:04:05"),
	)

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}
