package jobs

import (
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// LogBuffer is a thread-safe circular log buffer with subscriber support
type LogBuffer struct {
	entries   []LogEntry
	maxSize   int
	index     int
	full      bool
	mu        sync.RWMutex

	// Subscribers for real-time streaming
	subscribers map[chan LogEntry]bool
	subMu       sync.RWMutex
}

// NewLogBuffer creates a new log buffer with the specified maximum size
func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries:     make([]LogEntry, maxSize),
		maxSize:     maxSize,
		subscribers: make(map[chan LogEntry]bool),
	}
}

// Write adds a log entry to the buffer
func (lb *LogBuffer) Write(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	// Add to circular buffer
	lb.mu.Lock()
	lb.entries[lb.index] = entry
	lb.index = (lb.index + 1) % lb.maxSize
	if lb.index == 0 {
		lb.full = true
	}
	lb.mu.Unlock()

	// Notify subscribers (non-blocking)
	lb.subMu.RLock()
	defer lb.subMu.RUnlock()

	for ch := range lb.subscribers {
		select {
		case ch <- entry:
			// Successfully sent
		default:
			// Subscriber is slow, skip this entry to avoid blocking
		}
	}
}

// GetAll returns all log entries in chronological order
func (lb *LogBuffer) GetAll() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if !lb.full {
		// Buffer not full yet, return entries from start to current index
		result := make([]LogEntry, lb.index)
		copy(result, lb.entries[:lb.index])
		return result
	}

	// Buffer is full, return in correct order (oldest to newest)
	result := make([]LogEntry, lb.maxSize)
	copy(result[:lb.maxSize-lb.index], lb.entries[lb.index:])
	copy(result[lb.maxSize-lb.index:], lb.entries[:lb.index])
	return result
}

// GetSince returns log entries since the specified time
func (lb *LogBuffer) GetSince(since time.Time) []LogEntry {
	all := lb.GetAll()

	var result []LogEntry
	for _, entry := range all {
		if entry.Timestamp.After(since) {
			result = append(result, entry)
		}
	}

	return result
}

// Subscribe creates a channel that receives new log entries in real-time
func (lb *LogBuffer) Subscribe() chan LogEntry {
	lb.subMu.Lock()
	defer lb.subMu.Unlock()

	ch := make(chan LogEntry, 100) // Buffered to handle bursts
	lb.subscribers[ch] = true
	return ch
}

// Unsubscribe removes a subscriber channel
func (lb *LogBuffer) Unsubscribe(ch chan LogEntry) {
	lb.subMu.Lock()
	defer lb.subMu.Unlock()

	if _, exists := lb.subscribers[ch]; exists {
		delete(lb.subscribers, ch)
		close(ch)
	}
}

// SubscriberCount returns the number of active subscribers
func (lb *LogBuffer) SubscriberCount() int {
	lb.subMu.RLock()
	defer lb.subMu.RUnlock()
	return len(lb.subscribers)
}

// Clear removes all log entries (useful for testing)
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.entries = make([]LogEntry, lb.maxSize)
	lb.index = 0
	lb.full = false
}
