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
	default:
		return nil, fmt.Errorf("unsupported notifier type: %s", cfg.Type)
	}
}
