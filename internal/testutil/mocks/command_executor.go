package mocks

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// CommandExecutor is a mock for command execution used in tests
type CommandExecutor struct {
	mu sync.Mutex

	// Recorded calls for verification
	ExecuteCalls           []ExecuteCall
	ExecuteWithStderrCalls []ExecuteWithStderrCall
	ExecuteWithStdinCalls  []ExecuteWithStdinCall
	CheckCommandCalls      []string

	// Configurable responses
	ExecuteFunc           func(ctx context.Context, w io.Writer, name string, args ...string) error
	ExecuteWithStderrFunc func(ctx context.Context, w io.Writer, stderr io.Writer, name string, args ...string) error
	CheckCommandFunc      func(name string) error
	ExecuteWithStdinFunc  func(ctx context.Context, r io.Reader, name string, args ...string) error

	// Simple response configuration
	ExecuteOutput      string        // What to write to writer
	ExecuteError       error         // Error to return
	CheckCommandError  error         // Error for CheckCommandExists
	CommandsAvailable  map[string]bool // Which commands exist
}

// ExecuteCall records details about an ExecuteCommand call
type ExecuteCall struct {
	Name string
	Args []string
}

// ExecuteWithStderrCall records details about an ExecuteCommandWithStderr call
type ExecuteWithStderrCall struct {
	Name string
	Args []string
}

// ExecuteWithStdinCall records details about an ExecuteCommandWithStdin call
type ExecuteWithStdinCall struct {
	Name  string
	Args  []string
	Input string // Captured input data
}

// NewCommandExecutor creates a new mock command executor
func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{
		ExecuteCalls:           make([]ExecuteCall, 0),
		ExecuteWithStderrCalls: make([]ExecuteWithStderrCall, 0),
		ExecuteWithStdinCalls:  make([]ExecuteWithStdinCall, 0),
		CheckCommandCalls:      make([]string, 0),
		CommandsAvailable:      make(map[string]bool),
	}
}

// ExecuteCommand mocks the util.ExecuteCommand function
func (m *CommandExecutor) ExecuteCommand(ctx context.Context, w io.Writer, name string, args ...string) error {
	m.mu.Lock()
	m.ExecuteCalls = append(m.ExecuteCalls, ExecuteCall{
		Name: name,
		Args: append([]string{}, args...),
	})
	m.mu.Unlock()

	// Use custom function if provided
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, w, name, args...)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Write configured output
	if m.ExecuteOutput != "" {
		if _, err := io.WriteString(w, m.ExecuteOutput); err != nil {
			return err
		}
	}

	// Return configured error
	return m.ExecuteError
}

// ExecuteCommandWithStderr mocks the util.ExecuteCommandWithStderr function
func (m *CommandExecutor) ExecuteCommandWithStderr(ctx context.Context, w io.Writer, stderr io.Writer, name string, args ...string) error {
	m.mu.Lock()
	m.ExecuteWithStderrCalls = append(m.ExecuteWithStderrCalls, ExecuteWithStderrCall{
		Name: name,
		Args: append([]string{}, args...),
	})
	m.mu.Unlock()

	// Use custom function if provided
	if m.ExecuteWithStderrFunc != nil {
		return m.ExecuteWithStderrFunc(ctx, w, stderr, name, args...)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Write configured output
	if m.ExecuteOutput != "" {
		if _, err := io.WriteString(w, m.ExecuteOutput); err != nil {
			return err
		}
	}

	// Return configured error
	return m.ExecuteError
}

// CheckCommandExists mocks the util.CheckCommandExists function
func (m *CommandExecutor) CheckCommandExists(name string) error {
	m.mu.Lock()
	m.CheckCommandCalls = append(m.CheckCommandCalls, name)
	m.mu.Unlock()

	// Use custom function if provided
	if m.CheckCommandFunc != nil {
		return m.CheckCommandFunc(name)
	}

	// Check if command is in available list
	if len(m.CommandsAvailable) > 0 {
		if available, exists := m.CommandsAvailable[name]; exists && !available {
			return fmt.Errorf("command '%s' not found in PATH", name)
		}
		if _, exists := m.CommandsAvailable[name]; !exists {
			return fmt.Errorf("command '%s' not found in PATH", name)
		}
	}

	// Return configured error
	return m.CheckCommandError
}

// ExecuteCommandWithStdin mocks the util.ExecuteCommandWithStdin function
func (m *CommandExecutor) ExecuteCommandWithStdin(ctx context.Context, r io.Reader, name string, args ...string) error {
	// Capture input for verification
	input := &strings.Builder{}
	if r != nil {
		io.Copy(input, r)
	}

	m.mu.Lock()
	m.ExecuteWithStdinCalls = append(m.ExecuteWithStdinCalls, ExecuteWithStdinCall{
		Name:  name,
		Args:  append([]string{}, args...),
		Input: input.String(),
	})
	m.mu.Unlock()

	// Use custom function if provided
	if m.ExecuteWithStdinFunc != nil {
		// Create a new reader from the captured input
		return m.ExecuteWithStdinFunc(ctx, strings.NewReader(input.String()), name, args...)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Return configured error
	return m.ExecuteError
}

// Reset clears all recorded calls
func (m *CommandExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ExecuteCalls = make([]ExecuteCall, 0)
	m.ExecuteWithStderrCalls = make([]ExecuteWithStderrCall, 0)
	m.ExecuteWithStdinCalls = make([]ExecuteWithStdinCall, 0)
	m.CheckCommandCalls = make([]string, 0)
}

// GetLastExecuteCall returns the last ExecuteCommand call, or nil if none
func (m *CommandExecutor) GetLastExecuteCall() *ExecuteCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.ExecuteCalls) == 0 {
		return nil
	}
	return &m.ExecuteCalls[len(m.ExecuteCalls)-1]
}

// GetLastExecuteWithStdinCall returns the last ExecuteCommandWithStdin call, or nil if none
func (m *CommandExecutor) GetLastExecuteWithStdinCall() *ExecuteWithStdinCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.ExecuteWithStdinCalls) == 0 {
		return nil
	}
	return &m.ExecuteWithStdinCalls[len(m.ExecuteWithStdinCalls)-1]
}

// AssertExecuteCalledWith checks if ExecuteCommand was called with specific name and args
func (m *CommandExecutor) AssertExecuteCalledWith(name string, args ...string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.ExecuteCalls {
		if call.Name == name && equalStringSlices(call.Args, args) {
			return true
		}
	}
	return false
}

// AssertCheckCommandCalledWith checks if CheckCommandExists was called with specific name
func (m *CommandExecutor) AssertCheckCommandCalledWith(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.CheckCommandCalls {
		if call == name {
			return true
		}
	}
	return false
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
