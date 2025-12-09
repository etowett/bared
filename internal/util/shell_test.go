package util

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		wantErr     bool
		errContains string
		validate    func(*testing.T, string)
	}{
		{
			name:    "successful command - echo",
			command: getEchoCommand(),
			args:    []string{"hello world"},
			wantErr: false,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "hello world")
			},
		},
		{
			name:        "command not found",
			command:     "nonexistent-command-12345",
			args:        []string{},
			wantErr:     true,
			errContains: "command failed",
		},
		{
			name:    "command with multiple args",
			command: getEchoCommand(),
			args:    []string{"arg1", "arg2", "arg3"},
			wantErr: false,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "arg1")
				assert.Contains(t, output, "arg2")
				assert.Contains(t, output, "arg3")
			},
		},
		{
			name:        "command with error exit code",
			command:     getFalseCommand(),
			args:        []string{},
			wantErr:     true,
			errContains: "command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var buf bytes.Buffer

			err := ExecuteCommand(ctx, &buf, tt.command, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, buf.String())
				}
			}
		})
	}
}

func TestExecuteCommand_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	// Use a command that takes longer than the timeout
	err := ExecuteCommand(ctx, &buf, getSleepCommand(), "1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command failed")
}

func TestExecuteCommandWithStderr(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		args           []string
		wantErr        bool
		validateStdout func(*testing.T, string)
		validateStderr func(*testing.T, string)
	}{
		{
			name:    "command with stdout only",
			command: getEchoCommand(),
			args:    []string{"stdout message"},
			wantErr: false,
			validateStdout: func(t *testing.T, output string) {
				assert.Contains(t, output, "stdout message")
			},
			validateStderr: func(t *testing.T, output string) {
				assert.Empty(t, output)
			},
		},
		{
			name:    "command with stderr output",
			command: "sh",
			args:    []string{"-c", "echo 'error message' >&2"},
			wantErr: false,
			validateStderr: func(t *testing.T, output string) {
				assert.Contains(t, output, "error message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var stdout, stderr bytes.Buffer

			err := ExecuteCommandWithStderr(ctx, &stdout, &stderr, tt.command, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validateStdout != nil {
					tt.validateStdout(t, stdout.String())
				}
				if tt.validateStderr != nil {
					tt.validateStderr(t, stderr.String())
				}
			}
		})
	}
}

func TestExecuteCommandWithStderr_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var stdout, stderr bytes.Buffer
	err := ExecuteCommandWithStderr(ctx, &stdout, &stderr, getSleepCommand(), "1")

	assert.Error(t, err)
}

func TestCheckCommandExists(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "existing command - echo",
			command: "echo",
			wantErr: false,
		},
		{
			name:    "existing command - ls or dir",
			command: getLsCommand(),
			wantErr: false,
		},
		{
			name:        "non-existing command",
			command:     "nonexistent-command-xyz-12345",
			wantErr:     true,
			errContains: "not found in PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckCommandExists(tt.command)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecuteCommandWithStdin(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		input       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful command with stdin - cat",
			command: getCatCommand(),
			args:    []string{},
			input:   "test input data\n",
			wantErr: false,
		},
		{
			name:    "command with multiline input",
			command: getCatCommand(),
			args:    []string{},
			input:   "line1\nline2\nline3\n",
			wantErr: false,
		},
		{
			name:    "command with empty input",
			command: getCatCommand(),
			args:    []string{},
			input:   "",
			wantErr: false,
		},
		{
			name:        "command not found with stdin",
			command:     "nonexistent-command-xyz",
			args:        []string{},
			input:       "some input",
			wantErr:     true,
			errContains: "command failed",
		},
		{
			name:        "command with error exit code",
			command:     getFalseCommand(),
			args:        []string{},
			input:       "input",
			wantErr:     true,
			errContains: "command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader := strings.NewReader(tt.input)

			err := ExecuteCommandWithStdin(ctx, reader, tt.command, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecuteCommandWithStdin_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	reader := strings.NewReader("test input")
	err := ExecuteCommandWithStdin(ctx, reader, getSleepCommand(), "1")

	assert.Error(t, err)
}

func TestExecuteCommandWithStdin_LargeInput(t *testing.T) {
	// Test with a larger input to ensure streaming works
	largeInput := strings.Repeat("test data line\n", 1000)
	reader := strings.NewReader(largeInput)

	ctx := context.Background()
	err := ExecuteCommandWithStdin(ctx, reader, getCatCommand(), []string{}...)

	assert.NoError(t, err)
}

// Helper functions to get cross-platform commands

func getEchoCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "echo"
}

func getFalseCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "false"
}

func getSleepCommand() string {
	if runtime.GOOS == "windows" {
		return "timeout"
	}
	return "sleep"
}

func getLsCommand() string {
	if runtime.GOOS == "windows" {
		return "dir"
	}
	return "ls"
}

func getCatCommand() string {
	if runtime.GOOS == "windows" {
		return "findstr"
	}
	return "cat"
}

// TestExecuteCommand_RealWorldScenario tests a more realistic scenario
func TestExecuteCommand_RealWorldScenario(t *testing.T) {
	// Skip if mysqldump is not available
	if _, err := exec.LookPath("mysqldump"); err != nil {
		t.Skip("mysqldump not available, skipping real-world test")
	}

	ctx := context.Background()
	var buf bytes.Buffer

	// This will fail (no MySQL running), but tests that we can invoke mysqldump
	err := ExecuteCommand(ctx, &buf, "mysqldump", "--version")

	// We expect this to succeed (--version doesn't need MySQL connection)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "mysqldump")
}
