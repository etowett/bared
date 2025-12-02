package util

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	config := DefaultRetryConfig()
	attempts := 0

	err := Retry(context.Background(), config, func() error {
		attempts++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts, "should only attempt once on success")
}

func TestRetry_SuccessOnSecondAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	err := Retry(context.Background(), config, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts, "should succeed on second attempt")
}

func TestRetry_SuccessOnThirdAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	err := Retry(context.Background(), config, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts, "should succeed on third attempt")
}

func TestRetry_FailAfterMaxAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0
	expectedErr := errors.New("persistent error")

	err := Retry(context.Background(), config, func() error {
		attempts++
		return expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, 3, attempts, "should attempt max times")
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.ErrorIs(t, err, expectedErr)
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0
	attemptTimes := make([]time.Time, 0)

	start := time.Now()
	err := Retry(context.Background(), config, func() error {
		attempts++
		attemptTimes = append(attemptTimes, time.Now())
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, attempts)

	// Check that delays are increasing (exponential backoff)
	// First attempt: immediate
	// Second attempt: after ~50ms
	// Third attempt: after ~100ms more
	totalDuration := time.Since(start)

	// Total should be at least: 50ms + 100ms = 150ms
	assert.GreaterOrEqual(t, totalDuration.Milliseconds(), int64(140),
		"total duration should reflect exponential backoff")

	// Check individual delays
	if len(attemptTimes) >= 2 {
		firstDelay := attemptTimes[1].Sub(attemptTimes[0])
		assert.GreaterOrEqual(t, firstDelay.Milliseconds(), int64(40),
			"first delay should be approximately InitialDelay")
	}

	if len(attemptTimes) >= 3 {
		secondDelay := attemptTimes[2].Sub(attemptTimes[1])
		assert.GreaterOrEqual(t, secondDelay.Milliseconds(), int64(80),
			"second delay should be approximately InitialDelay * Multiplier")
	}
}

func TestRetry_MaxDelayLimit(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     30 * time.Millisecond, // Cap at 30ms
		Multiplier:   4.0,                   // Aggressive multiplier
	}
	attempts := 0
	attemptTimes := make([]time.Time, 0)

	Retry(context.Background(), config, func() error {
		attempts++
		attemptTimes = append(attemptTimes, time.Now())
		return errors.New("error")
	})

	// With multiplier 4.0:
	// Delay 1: 10ms
	// Delay 2: 40ms -> capped at 30ms
	// Delay 3: 160ms -> capped at 30ms
	// Delay 4: 640ms -> capped at 30ms

	// Check that later delays don't exceed MaxDelay
	if len(attemptTimes) >= 4 {
		delay := attemptTimes[3].Sub(attemptTimes[2])
		assert.LessOrEqual(t, delay.Milliseconds(), int64(50),
			"delay should be capped at MaxDelay")
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after first attempt
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, config, func() error {
		attempts++
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry cancelled")
	assert.LessOrEqual(t, attempts, 2, "should stop early due to cancellation")
}

func TestRetry_ContextCancellationImmediate(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := Retry(ctx, config, func() error {
		attempts++
		return errors.New("error")
	})

	assert.Error(t, err)
	// First attempt should execute, then cancellation should be detected
	assert.Equal(t, 1, attempts)
	assert.Contains(t, err.Error(), "retry cancelled")
}

func TestRetry_ContextTimeout(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	err := Retry(ctx, config, func() error {
		attempts++
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry cancelled")
	// Should get 1-2 attempts before timeout
	assert.LessOrEqual(t, attempts, 3, "should timeout before max attempts")
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestRetry_SingleAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	err := Retry(context.Background(), config, func() error {
		attempts++
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts, "should only attempt once")
	assert.Contains(t, err.Error(), "after 1 attempts")
}

func TestRetry_ZeroInitialDelay(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 0, // No delay
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	attempts := 0

	start := time.Now()
	err := Retry(context.Background(), config, func() error {
		attempts++
		return errors.New("error")
	})
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, 3, attempts)
	// Should complete quickly with no delays
	assert.Less(t, duration.Milliseconds(), int64(50),
		"should complete quickly with zero delay")
}

func TestRetry_RealWorldScenario_NetworkRetry(t *testing.T) {
	// Simulate a network operation that fails twice then succeeds
	config := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	attempts := 0
	simulatedNetworkErrors := []error{
		errors.New("connection refused"),
		errors.New("timeout"),
		nil, // Success on third attempt
	}

	err := Retry(context.Background(), config, func() error {
		if attempts < len(simulatedNetworkErrors) {
			err := simulatedNetworkErrors[attempts]
			attempts++
			return err
		}
		attempts++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts, "should succeed after 2 failures")
}

func TestRetry_DifferentErrorTypes(t *testing.T) {
	tests := []struct {
		name      string
		errors    []error
		wantRetry int
	}{
		{
			name: "all same error - all retries fail",
			errors: []error{
				errors.New("error1"),
				errors.New("error1"),
				errors.New("error1"),
				errors.New("error1"),
				errors.New("error1"),
			},
			wantRetry: 5,
		},
		{
			name: "different errors - all retries fail",
			errors: []error{
				errors.New("connection error"),
				errors.New("timeout error"),
				errors.New("permission error"),
				errors.New("network error"),
				errors.New("final error"),
			},
			wantRetry: 5,
		},
		{
			name: "success after one error",
			errors: []error{
				errors.New("temporary error"),
				nil,
			},
			wantRetry: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RetryConfig{
				MaxAttempts:  5,
				InitialDelay: 5 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   2.0,
			}

			attempts := 0
			err := Retry(context.Background(), config, func() error {
				currentAttempt := attempts
				attempts++
				if currentAttempt < len(tt.errors) {
					return tt.errors[currentAttempt]
				}
				return nil
			})

			require.Equal(t, tt.wantRetry, attempts)
			if tt.errors[len(tt.errors)-1] == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
