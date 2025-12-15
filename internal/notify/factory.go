// Package notify provides notification implementations for Slack and Discord.
package notify

import (
	"fmt"

	"bared/internal/config"
)

// New creates a new notifier based on the configuration
func New(cfg *config.Notifier) (Notifier, error) {
	switch cfg.Type {
	case "slack":
		return NewSlack(cfg), nil
	case "email":
		return NewEmail(cfg), nil
	case "webhook":
		return NewWebhook(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported notifier type: %s", cfg.Type)
	}
}
