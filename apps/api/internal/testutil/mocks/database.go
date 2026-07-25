package mocks

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"bared/internal/database"
)

// MockDumper is a mock implementation of the database.Dumper interface
type MockDumper struct {
	mu sync.Mutex

	// Recorded calls for verification
	DumpCalls     []DumpCall
	ValidateCalls int

	// Configurable responses
	DumpFunc     func(ctx context.Context, w io.Writer) (*database.DumpMetadata, error)
	ValidateFunc func(ctx context.Context) error

	// Simple response configuration
	NameValue     string
	DumpOutput    string // Data to write when Dump is called
	DumpMetadata  *database.DumpMetadata
	DumpError     error
	ValidateError error
}

// DumpCall records details about a Dump call
type DumpCall struct {
	Timestamp time.Time
}

// MockRestorer is a mock implementation of the database.Restorer interface
type MockRestorer struct {
	mu sync.Mutex

	// Recorded calls for verification
	RestoreCalls []RestoreCall

	// Configurable responses
	RestoreFunc func(ctx context.Context, r io.Reader) error

	// Simple response configuration
	NameValue    string
	RestoreError error
}

// RestoreCall records details about a Restore call
type RestoreCall struct {
	Timestamp time.Time
	Input     string // Captured input data
}

// NewMockDumper creates a new mock dumper
func NewMockDumper(name string) *MockDumper {
	return &MockDumper{
		NameValue:  name,
		DumpCalls:  make([]DumpCall, 0),
		DumpOutput: "mock dump data\n",
		DumpMetadata: &database.DumpMetadata{
			DatabaseName: "test_db",
			DatabaseType: "mysql",
			Size:         100,
			Duration:     1 * time.Second,
			Timestamp:    time.Now(),
		},
	}
}

// NewMockRestorer creates a new mock restorer
func NewMockRestorer(name string) *MockRestorer {
	return &MockRestorer{
		NameValue:    name,
		RestoreCalls: make([]RestoreCall, 0),
	}
}

// Dump mocks the database.Dump method
func (m *MockDumper) Dump(ctx context.Context, w io.Writer) (*database.DumpMetadata, error) {
	m.mu.Lock()
	m.DumpCalls = append(m.DumpCalls, DumpCall{
		Timestamp: time.Now(),
	})
	m.mu.Unlock()

	// Use custom function if provided
	if m.DumpFunc != nil {
		return m.DumpFunc(ctx, w)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Write configured output
	if m.DumpOutput != "" {
		if _, err := io.WriteString(w, m.DumpOutput); err != nil {
			return nil, err
		}
	}

	// Return configured error
	if m.DumpError != nil {
		return nil, m.DumpError
	}

	// Return configured metadata
	return m.DumpMetadata, nil
}

// Name mocks the database.Name method for dumper
func (m *MockDumper) Name() string {
	if m.NameValue == "" {
		return "mock-dumper"
	}
	return m.NameValue
}

// Validate mocks the database.Validate method
func (m *MockDumper) Validate(ctx context.Context) error {
	m.mu.Lock()
	m.ValidateCalls++
	m.mu.Unlock()

	// Use custom function if provided
	if m.ValidateFunc != nil {
		return m.ValidateFunc(ctx)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Return configured error
	return m.ValidateError
}

// Restore mocks the database.Restore method
func (m *MockRestorer) Restore(ctx context.Context, r io.Reader) error {
	// Capture input for verification
	input := &strings.Builder{}
	if r != nil {
		//nolint:errcheck // Error copying in mock is not critical
		_, _ = io.Copy(input, r)
	}

	m.mu.Lock()
	m.RestoreCalls = append(m.RestoreCalls, RestoreCall{
		Timestamp: time.Now(),
		Input:     input.String(),
	})
	m.mu.Unlock()

	// Use custom function if provided
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, strings.NewReader(input.String()))
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Return configured error
	return m.RestoreError
}

// Name mocks the database.Name method for restorer
func (m *MockRestorer) Name() string {
	if m.NameValue == "" {
		return "mock-restorer"
	}
	return m.NameValue
}

// Reset clears all recorded calls for dumper
func (m *MockDumper) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DumpCalls = make([]DumpCall, 0)
	m.ValidateCalls = 0
}

// Reset clears all recorded calls for restorer
func (m *MockRestorer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RestoreCalls = make([]RestoreCall, 0)
}

// GetLastDumpCall returns the last Dump call, or nil if none
func (m *MockDumper) GetLastDumpCall() *DumpCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.DumpCalls) == 0 {
		return nil
	}
	return &m.DumpCalls[len(m.DumpCalls)-1]
}

// GetLastRestoreCall returns the last Restore call, or nil if none
func (m *MockRestorer) GetLastRestoreCall() *RestoreCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.RestoreCalls) == 0 {
		return nil
	}
	return &m.RestoreCalls[len(m.RestoreCalls)-1]
}

// DumpCallCount returns the number of times Dump was called
func (m *MockDumper) DumpCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.DumpCalls)
}

// RestoreCallCount returns the number of times Restore was called
func (m *MockRestorer) RestoreCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.RestoreCalls)
}

// ValidateCallCount returns the number of times Validate was called
func (m *MockDumper) ValidateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ValidateCalls
}

// MockDumperRestorer implements both Dumper and Restorer interfaces
type MockDumperRestorer struct {
	*MockDumper
	*MockRestorer
}

// NewMockDumperRestorer creates a mock that implements both interfaces
func NewMockDumperRestorer(name string) *MockDumperRestorer {
	return &MockDumperRestorer{
		MockDumper:   NewMockDumper(name),
		MockRestorer: NewMockRestorer(name),
	}
}
