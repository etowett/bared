package database

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"bared/internal/config"
	"bared/internal/util"
)

// Redis implements Dumper and Restorer for Redis databases
type Redis struct {
	conn *config.Connection
}

// NewRedis creates a new Redis dumper
func NewRedis(conn *config.Connection) *Redis {
	return &Redis{
		conn: conn,
	}
}

// Name returns the database name
func (r *Redis) Name() string {
	return fmt.Sprintf("redis:%s:%d", r.conn.Host, r.conn.Port)
}

// Validate checks if redis-cli command exists
func (r *Redis) Validate(_ context.Context) error {
	logger := util.GetLogger()

	if err := util.CheckCommandExists("redis-cli"); err != nil {
		logger.ErrorS("Failed to detect Redis dump command",
			"component", "redis",
			"host", r.conn.Host,
			"port", r.conn.Port,
			"command", "redis-cli",
			"error", err)
		return fmt.Errorf("redis-cli not found: %w (install redis-tools package)", err)
	}

	logger.InfoS("Redis dump command detected",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"command", "redis-cli")

	return nil
}

// Dump executes redis backup using redis-cli --rdb
func (r *Redis) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	logger := util.GetLogger()
	startTime := time.Now()

	// Create a temporary file for the RDB dump
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("redis-dump-%d.rdb", time.Now().Unix()))
	defer func() {
		//nolint:errcheck // Error removing temp file during cleanup is not critical
		_ = os.Remove(tmpFile)
	}()

	logger.InfoS("Creating temporary RDB file",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"tmpfile", tmpFile)

	// Use redis-cli --rdb to dump the database
	args := r.buildDumpArgs(tmpFile)
	sanitizedArgs := r.sanitizeArgs(args)

	logger.InfoS("Starting Redis dump",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"command", "redis-cli",
		"args", sanitizedArgs)

	// Execute redis-cli --rdb (it writes to the specified file)
	if err := util.ExecuteCommand(ctx, io.Discard, "redis-cli", args...); err != nil {
		logger.ErrorS("Redis dump failed",
			"component", "redis",
			"host", r.conn.Host,
			"port", r.conn.Port,
			"command", "redis-cli",
			"duration", time.Since(startTime).String(),
			"error", err)
		return nil, fmt.Errorf("redis-cli --rdb failed: %w", err)
	}

	logger.InfoS("Redis RDB dump completed, reading file",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"tmpfile", tmpFile)

	// Read the RDB file and write to the output writer
	rdbFile, err := os.Open(tmpFile) // #nosec G304 - tmpFile is constructed from os.TempDir()
	if err != nil {
		logger.ErrorS("Failed to open RDB file",
			"component", "redis",
			"tmpfile", tmpFile,
			"error", err)
		return nil, fmt.Errorf("failed to open RDB file: %w", err)
	}
	defer func() {
		//nolint:errcheck // Error closing RDB file during cleanup is not critical
		_ = rdbFile.Close()
	}()

	size, err := io.Copy(w, rdbFile)
	if err != nil {
		logger.ErrorS("Failed to copy RDB data",
			"component", "redis",
			"tmpfile", tmpFile,
			"error", err)
		return nil, fmt.Errorf("failed to copy RDB data: %w", err)
	}

	duration := time.Since(startTime)
	logger.InfoS("Redis dump completed successfully",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"command", "redis-cli",
		"size_bytes", size,
		"duration", duration.String())

	metadata := &DumpMetadata{
		DatabaseName: fmt.Sprintf("%s:%d", r.conn.Host, r.conn.Port),
		DatabaseType: "redis",
		Size:         size,
		Duration:     duration,
		Timestamp:    startTime,
	}

	return metadata, nil
}

// Restore is not implemented for Redis (requires stopping server and replacing RDB file)
func (r *Redis) Restore(_ context.Context, _ io.Reader) error {
	logger := util.GetLogger()

	logger.WarnS("Redis restore not implemented",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"reason", "requires manual RDB file replacement")

	return fmt.Errorf("redis restore not yet implemented (requires manual RDB file replacement)")
}

// ValidateConnection tests Redis connectivity
func (r *Redis) ValidateConnection(ctx context.Context) error {
	logger := util.GetLogger()

	// Check if redis-cli command exists
	if err := util.CheckCommandExists("redis-cli"); err != nil {
		logger.ErrorS("Failed to detect Redis restore command",
			"component", "redis",
			"host", r.conn.Host,
			"port", r.conn.Port,
			"command", "redis-cli",
			"error", err)
		return fmt.Errorf("redis-cli not found: %w (install redis-tools package)", err)
	}

	logger.InfoS("Redis restore command detected",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"command", "redis-cli")

	// Build test connection args
	args := []string{
		"-h", r.conn.Host,
		"-p", fmt.Sprintf("%d", r.conn.Port),
	}

	passwordSet := false
	if r.conn.Password != "" {
		args = append(args, "-a", r.conn.Password)
		passwordSet = true
	}

	args = append(args, "PING")

	logger.InfoS("Testing Redis connection",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port,
		"command", "redis-cli PING",
		"password_set", passwordSet)

	if err := util.ExecuteCommand(ctx, io.Discard, "redis-cli", args...); err != nil {
		logger.ErrorS("Redis connection test failed",
			"component", "redis",
			"host", r.conn.Host,
			"port", r.conn.Port,
			"error", err)
		return fmt.Errorf("redis connection failed: %w", err)
	}

	logger.InfoS("Redis connection test successful",
		"component", "redis",
		"host", r.conn.Host,
		"port", r.conn.Port)

	return nil
}

// sanitizeArgs removes sensitive information from command arguments for logging
func (r *Redis) sanitizeArgs(args []string) []string {
	sanitized := make([]string, len(args))
	redactNext := false
	for i, arg := range args {
		if redactNext {
			sanitized[i] = "***REDACTED***"
			redactNext = false
		} else if arg == "-a" || arg == "--auth" {
			sanitized[i] = arg
			redactNext = true
		} else {
			sanitized[i] = arg
		}
	}
	return sanitized
}

func (r *Redis) buildDumpArgs(outputFile string) []string {
	args := []string{
		"-h", r.conn.Host,
		"-p", fmt.Sprintf("%d", r.conn.Port),
	}

	if r.conn.Password != "" {
		args = append(args, "-a", r.conn.Password)
	}

	// Use --rdb to dump to file
	args = append(args, "--rdb", outputFile)

	return args
}
