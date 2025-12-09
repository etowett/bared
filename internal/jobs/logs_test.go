package jobs

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogBuffer(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int
	}{
		{
			name:    "small buffer",
			maxSize: 10,
		},
		{
			name:    "medium buffer",
			maxSize: 100,
		},
		{
			name:    "large buffer",
			maxSize: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLogBuffer(tt.maxSize)

			require.NotNil(t, lb)
			assert.Equal(t, tt.maxSize, lb.maxSize)
			assert.Equal(t, 0, lb.index)
			assert.False(t, lb.full)
			assert.NotNil(t, lb.subscribers)
			assert.Equal(t, 0, len(lb.subscribers))
		})
	}
}

func TestLogBuffer_Write(t *testing.T) {
	lb := NewLogBuffer(10)

	tests := []struct {
		level   string
		message string
	}{
		{"INFO", "Starting backup"},
		{"DEBUG", "Connecting to database"},
		{"WARN", "Connection slow"},
		{"ERROR", "Failed to connect"},
	}

	for i, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			beforeWrite := time.Now()
			lb.Write(tt.level, tt.message)
			afterWrite := time.Now()

			entries := lb.GetAll()
			require.Len(t, entries, i+1)

			entry := entries[i]
			assert.Equal(t, tt.level, entry.Level)
			assert.Equal(t, tt.message, entry.Message)
			assert.True(t, entry.Timestamp.After(beforeWrite) || entry.Timestamp.Equal(beforeWrite))
			assert.True(t, entry.Timestamp.Before(afterWrite) || entry.Timestamp.Equal(afterWrite))
		})
	}
}

func TestLogBuffer_GetAll_NotFull(t *testing.T) {
	lb := NewLogBuffer(10)

	// Write 5 entries (buffer not full)
	for i := 0; i < 5; i++ {
		lb.Write("INFO", "Message "+string(rune('A'+i)))
	}

	entries := lb.GetAll()
	require.Len(t, entries, 5)

	for i, entry := range entries {
		assert.Equal(t, "INFO", entry.Level)
		assert.Equal(t, "Message "+string(rune('A'+i)), entry.Message)
	}
}

func TestLogBuffer_GetAll_Full(t *testing.T) {
	lb := NewLogBuffer(5)

	// Write 10 entries (buffer will wrap around)
	for i := 0; i < 10; i++ {
		lb.Write("INFO", "Message "+string(rune('A'+i)))
		time.Sleep(1 * time.Millisecond) // Ensure unique timestamps
	}

	entries := lb.GetAll()
	require.Len(t, entries, 5, "should only have 5 entries (buffer size)")

	// Should have the last 5 messages (F, G, H, I, J)
	expectedMessages := []string{"Message F", "Message G", "Message H", "Message I", "Message J"}
	for i, entry := range entries {
		assert.Equal(t, expectedMessages[i], entry.Message)
	}
}

func TestLogBuffer_CircularBufferOverflow(t *testing.T) {
	lb := NewLogBuffer(3)

	// Write more entries than buffer size
	messages := []string{"First", "Second", "Third", "Fourth", "Fifth"}
	for _, msg := range messages {
		lb.Write("INFO", msg)
		time.Sleep(1 * time.Millisecond)
	}

	entries := lb.GetAll()
	require.Len(t, entries, 3, "buffer should maintain max size")

	// Should have the last 3 messages
	assert.Equal(t, "Third", entries[0].Message)
	assert.Equal(t, "Fourth", entries[1].Message)
	assert.Equal(t, "Fifth", entries[2].Message)
}

func TestLogBuffer_GetSince(t *testing.T) {
	lb := NewLogBuffer(10)

	// Write entries with delays
	now := time.Now()
	lb.Write("INFO", "Message 1")
	time.Sleep(10 * time.Millisecond)

	midpoint := time.Now()
	lb.Write("INFO", "Message 2")
	time.Sleep(10 * time.Millisecond)

	lb.Write("INFO", "Message 3")

	tests := []struct {
		name          string
		since         time.Time
		expectedCount int
	}{
		{
			name:          "since beginning",
			since:         now.Add(-1 * time.Second),
			expectedCount: 3,
		},
		{
			name:          "since midpoint",
			since:         midpoint,
			expectedCount: 2,
		},
		{
			name:          "since future",
			since:         time.Now().Add(1 * time.Second),
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := lb.GetSince(tt.since)
			assert.Len(t, entries, tt.expectedCount)
		})
	}
}

func TestLogBuffer_Subscribe(t *testing.T) {
	lb := NewLogBuffer(10)

	ch := lb.Subscribe()
	require.NotNil(t, ch)
	assert.Equal(t, 1, lb.SubscriberCount())

	// Write a log entry
	lb.Write("INFO", "Test message")

	// Should receive on channel
	select {
	case entry := <-ch:
		assert.Equal(t, "INFO", entry.Level)
		assert.Equal(t, "Test message", entry.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive log entry on subscribed channel")
	}

	lb.Unsubscribe(ch)
	assert.Equal(t, 0, lb.SubscriberCount())
}

func TestLogBuffer_Unsubscribe(t *testing.T) {
	lb := NewLogBuffer(10)

	ch := lb.Subscribe()
	assert.Equal(t, 1, lb.SubscriberCount())

	lb.Unsubscribe(ch)
	assert.Equal(t, 0, lb.SubscriberCount())

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after unsubscribe")

	// Unsubscribing again should be safe
	lb.Unsubscribe(ch)
	assert.Equal(t, 0, lb.SubscriberCount())
}

func TestLogBuffer_MultipleSubscribers(t *testing.T) {
	lb := NewLogBuffer(10)

	// Create multiple subscribers
	ch1 := lb.Subscribe()
	ch2 := lb.Subscribe()
	ch3 := lb.Subscribe()

	assert.Equal(t, 3, lb.SubscriberCount())

	// Write a message
	lb.Write("INFO", "Broadcast message")

	// All subscribers should receive it
	subscribers := []chan LogEntry{ch1, ch2, ch3}
	for i, ch := range subscribers {
		select {
		case entry := <-ch:
			assert.Equal(t, "Broadcast message", entry.Message, "subscriber %d should receive message", i+1)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d did not receive message", i+1)
		}
	}

	// Unsubscribe one
	lb.Unsubscribe(ch2)
	assert.Equal(t, 2, lb.SubscriberCount())

	// Write another message
	lb.Write("INFO", "Second message")

	// Only remaining subscribers should receive it
	select {
	case <-ch1:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 1 should still receive messages")
	}

	select {
	case <-ch3:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 3 should still receive messages")
	}

	// Unsubscribe all
	lb.Unsubscribe(ch1)
	lb.Unsubscribe(ch3)
	assert.Equal(t, 0, lb.SubscriberCount())
}

func TestLogBuffer_ConcurrentWrites(t *testing.T) {
	lb := NewLogBuffer(100)

	var wg sync.WaitGroup
	writers := 10
	messagesPerWriter := 20

	// Concurrent writes
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < messagesPerWriter; j++ {
				lb.Write("INFO", "Writer "+string(rune('A'+writerID)))
			}
		}(i)
	}

	wg.Wait()

	// Should have written many entries without panicking
	entries := lb.GetAll()
	assert.LessOrEqual(t, len(entries), 100, "should not exceed buffer size")
	assert.Greater(t, len(entries), 0, "should have some entries")
}

func TestLogBuffer_ConcurrentSubscribers(t *testing.T) {
	lb := NewLogBuffer(100)

	var wg sync.WaitGroup
	subscriberCount := 10

	// Create subscribers concurrently
	channels := make([]chan LogEntry, subscriberCount)
	for i := 0; i < subscriberCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			channels[idx] = lb.Subscribe()
		}(i)
	}

	wg.Wait()
	assert.Equal(t, subscriberCount, lb.SubscriberCount())

	// Write a message
	lb.Write("INFO", "Test message")

	// All subscribers should receive (or skip if slow)
	time.Sleep(10 * time.Millisecond) // Give time for delivery

	// Unsubscribe all concurrently
	for i := 0; i < subscriberCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lb.Unsubscribe(channels[idx])
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 0, lb.SubscriberCount())
}

func TestLogBuffer_ConcurrentReadWrite(t *testing.T) {
	lb := NewLogBuffer(50)

	var wg sync.WaitGroup
	duration := 100 * time.Millisecond
	stopTime := time.Now().Add(duration)

	// Concurrent writers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for time.Now().Before(stopTime) {
				lb.Write("INFO", "Writer "+string(rune('A'+writerID)))
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stopTime) {
				_ = lb.GetAll()
			}
		}()
	}

	// Concurrent subscriber management
	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Now().Before(stopTime) {
			ch := lb.Subscribe()
			time.Sleep(5 * time.Millisecond)
			lb.Unsubscribe(ch)
		}
	}()

	wg.Wait()

	// Should complete without panicking or data races
	entries := lb.GetAll()
	assert.LessOrEqual(t, len(entries), 50)
}

func TestLogBuffer_Clear(t *testing.T) {
	lb := NewLogBuffer(10)

	// Add some entries
	for i := 0; i < 5; i++ {
		lb.Write("INFO", "Message "+string(rune('A'+i)))
	}

	entries := lb.GetAll()
	assert.Len(t, entries, 5)

	// Clear
	lb.Clear()

	// Should be empty
	entries = lb.GetAll()
	assert.Len(t, entries, 0)
	assert.Equal(t, 0, lb.index)
	assert.False(t, lb.full)
}

func TestLogBuffer_SlowSubscriber(t *testing.T) {
	lb := NewLogBuffer(10)

	// Create a subscriber with a small buffer
	ch := make(chan LogEntry, 1)
	lb.subMu.Lock()
	lb.subscribers[ch] = true
	lb.subMu.Unlock()

	// Fill the subscriber's buffer
	lb.Write("INFO", "Message 1")

	// Subscriber buffer is now full, next write should not block
	done := make(chan bool)
	go func() {
		lb.Write("INFO", "Message 2")
		done <- true
	}()

	// Should complete quickly (not block on slow subscriber)
	select {
	case <-done:
		// Expected - write should not block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("write blocked on slow subscriber")
	}

	// Cleanup
	lb.Unsubscribe(ch)
}

func TestLogBuffer_SubscriberCount(t *testing.T) {
	lb := NewLogBuffer(10)

	assert.Equal(t, 0, lb.SubscriberCount())

	ch1 := lb.Subscribe()
	assert.Equal(t, 1, lb.SubscriberCount())

	ch2 := lb.Subscribe()
	assert.Equal(t, 2, lb.SubscriberCount())

	ch3 := lb.Subscribe()
	assert.Equal(t, 3, lb.SubscriberCount())

	lb.Unsubscribe(ch2)
	assert.Equal(t, 2, lb.SubscriberCount())

	lb.Unsubscribe(ch1)
	lb.Unsubscribe(ch3)
	assert.Equal(t, 0, lb.SubscriberCount())
}

func TestLogBuffer_ChronologicalOrder(t *testing.T) {
	lb := NewLogBuffer(5)

	// Write entries with slight delays to ensure order
	messages := []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth"}
	for _, msg := range messages {
		lb.Write("INFO", msg)
		time.Sleep(2 * time.Millisecond)
	}

	entries := lb.GetAll()
	require.Len(t, entries, 5)

	// Verify chronological order
	for i := 0; i < len(entries)-1; i++ {
		assert.True(t, entries[i].Timestamp.Before(entries[i+1].Timestamp),
			"entries should be in chronological order")
	}

	// Should have last 5 messages
	assert.Equal(t, "Second", entries[0].Message)
	assert.Equal(t, "Sixth", entries[4].Message)
}

func TestLogEntry_Fields(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "ERROR",
		Message:   "Test error message",
	}

	assert.NotZero(t, entry.Timestamp)
	assert.Equal(t, "ERROR", entry.Level)
	assert.Equal(t, "Test error message", entry.Message)
}
