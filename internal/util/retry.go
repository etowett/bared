package util

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// Retry executes fn with exponential backoff
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			// Log success if this wasn't the first attempt
			if attempt > 1 {
				Info("Operation succeeded after %d attempts", attempt)
			}
			return nil
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == config.MaxAttempts {
			Error("Operation failed after %d attempts: %v", attempt, err)
			break
		}

		// Log retry with delay info
		Warn("Attempt %d/%d failed: %v - retrying in %v", attempt, config.MaxAttempts, err, delay)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			Debug("Delay elapsed, starting attempt %d", attempt+1)
		}

		// Increase delay for next attempt (exponential backoff)
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("after %d attempts: %w", config.MaxAttempts, lastErr)
}
