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
	if err := util.CheckCommandExists("redis-cli"); err != nil {
		return fmt.Errorf("redis-cli not found: %w (install redis-tools package)", err)
	}
	return nil
}

// Dump executes redis backup using redis-cli --rdb
func (r *Redis) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	startTime := time.Now()

	// Create a temporary file for the RDB dump
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("redis-dump-%d.rdb", time.Now().Unix()))
	defer func() {
		//nolint:errcheck // Error removing temp file during cleanup is not critical
		_ = os.Remove(tmpFile)
	}()

	// Use redis-cli --rdb to dump the database
	args := r.buildDumpArgs(tmpFile)

	// Execute redis-cli --rdb (it writes to the specified file)
	if err := util.ExecuteCommand(ctx, io.Discard, "redis-cli", args...); err != nil {
		return nil, fmt.Errorf("redis-cli --rdb failed: %w", err)
	}

	// Read the RDB file and write to the output writer
	rdbFile, err := os.Open(tmpFile) // #nosec G304 - tmpFile is constructed from os.TempDir()
	if err != nil {
		return nil, fmt.Errorf("failed to open RDB file: %w", err)
	}
	defer func() {
		//nolint:errcheck // Error closing RDB file during cleanup is not critical
		_ = rdbFile.Close()
	}()

	size, err := io.Copy(w, rdbFile)
	if err != nil {
		return nil, fmt.Errorf("failed to copy RDB data: %w", err)
	}

	metadata := &DumpMetadata{
		DatabaseName: fmt.Sprintf("%s:%d", r.conn.Host, r.conn.Port),
		DatabaseType: "redis",
		Size:         size,
		Duration:     time.Since(startTime),
		Timestamp:    startTime,
	}

	return metadata, nil
}

// Restore is not implemented for Redis (requires stopping server and replacing RDB file)
func (r *Redis) Restore(_ context.Context, _ io.Reader) error {
	return fmt.Errorf("redis restore not yet implemented (requires manual RDB file replacement)")
}

// ValidateConnection tests Redis connectivity
func (r *Redis) ValidateConnection(ctx context.Context) error {
	// Check if redis-cli command exists
	if err := util.CheckCommandExists("redis-cli"); err != nil {
		return fmt.Errorf("redis-cli not found: %w (install redis-tools package)", err)
	}

	// Build test connection args
	args := []string{
		"-h", r.conn.Host,
		"-p", fmt.Sprintf("%d", r.conn.Port),
	}

	if r.conn.Password != "" {
		args = append(args, "-a", r.conn.Password)
	}

	args = append(args, "PING")

	if err := util.ExecuteCommand(ctx, io.Discard, "redis-cli", args...); err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	return nil
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
