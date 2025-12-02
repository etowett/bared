package util

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// ExecuteCommand runs a command and writes output to the writer
func ExecuteCommand(ctx context.Context, w io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = w
	cmd.Stderr = io.Discard // Discard stderr to avoid polluting backup files

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}

	return nil
}

// ExecuteCommandWithStderr runs a command and captures both stdout and stderr
func ExecuteCommandWithStderr(ctx context.Context, w io.Writer, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = w
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %v: %w", name, args, err)
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

// ExecuteCommandWithStdin runs a command with input from reader
func ExecuteCommandWithStdin(ctx context.Context, r io.Reader, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = r
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}

	return nil
}
