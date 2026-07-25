package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ExecuteCommand runs a command and writes output to the writer
func ExecuteCommand(ctx context.Context, w io.Writer, name string, args ...string) error {
	return ExecuteCommandWithEnv(ctx, w, nil, name, args...)
}

// ExecuteCommandWithEnv runs a command with custom environment variables
func ExecuteCommandWithEnv(ctx context.Context, w io.Writer, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = w

	// Set environment variables if provided
	if env != nil {
		cmd.Env = os.Environ() // Start with current environment
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Capture stderr to include in error messages
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		stderrOutput := stderrBuf.String()
		if stderrOutput != "" {
			return fmt.Errorf("command failed: %s %v (exit code %d): %s: %w",
				name, args, exitCode, stderrOutput, err)
		}
		return fmt.Errorf("command failed: %s %v (exit code %d): %w",
			name, args, exitCode, err)
	}

	return nil
}

// ExecuteCommandWithStderr runs a command and captures both stdout and stderr
func ExecuteCommandWithStderr(ctx context.Context, w io.Writer, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = w
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		// Extract exit code if available
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		return fmt.Errorf("command failed: %s %v (exit code %d): %w", name, args, exitCode, err)
	}

	return nil
}

// CheckCommandExists checks if a command is available in PATH
func CheckCommandExists(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH: %w", name, err)
	}
	return nil
}

// DetectCommand checks for the first available command from a list of candidates
// Returns the first command found in PATH, or an error if none are available
func DetectCommand(candidates ...string) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no command candidates provided")
	}

	for _, cmd := range candidates {
		if err := CheckCommandExists(cmd); err == nil {
			return cmd, nil
		}
	}

	return "", fmt.Errorf("none of the required commands found: %v", candidates)
}

// ExecuteCommandWithStdin runs a command with input from reader
func ExecuteCommandWithStdin(ctx context.Context, r io.Reader, name string, args ...string) error {
	return ExecuteCommandWithStdinAndEnv(ctx, r, nil, name, args...)
}

// ExecuteCommandWithStdinAndEnv runs a command with input from reader and custom environment variables
func ExecuteCommandWithStdinAndEnv(ctx context.Context, r io.Reader, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = r
	cmd.Stdout = io.Discard

	// Set environment variables if provided
	if env != nil {
		cmd.Env = os.Environ() // Start with current environment
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Capture stderr to include in error messages
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		// Extract exit code if available
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		// Build detailed error message
		stderrOutput := stderrBuf.String()
		if stderrOutput != "" {
			return fmt.Errorf("command failed: %s %v (exit code %d): %s: %w",
				name, args, exitCode, stderrOutput, err)
		}
		return fmt.Errorf("command failed: %s %v (exit code %d): %w",
			name, args, exitCode, err)
	}

	return nil
}

// ExecuteCommandOutput runs a command and returns its stdout output
func ExecuteCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return ExecuteCommandOutputWithEnv(ctx, nil, name, args...)
}

// ExecuteCommandOutputWithEnv runs a command with environment variables and returns stdout output
func ExecuteCommandOutputWithEnv(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// Set environment variables if provided
	if env != nil {
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Capture both stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		stderrOutput := stderrBuf.String()
		if stderrOutput != "" {
			return nil, fmt.Errorf("command failed: %s %v (exit code %d): %s: %w",
				name, args, exitCode, stderrOutput, err)
		}
		return nil, fmt.Errorf("command failed: %s %v (exit code %d): %w",
			name, args, exitCode, err)
	}

	return stdoutBuf.Bytes(), nil
}
