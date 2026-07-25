// Package progress provides progress tracking and estimation for long-running operations.
package progress

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"bared/internal/config"
	"bared/internal/util"
)

// EstimateDatabaseSize estimates the size of a database before backup
func EstimateDatabaseSize(ctx context.Context, conn *config.Connection) (int64, error) {
	switch conn.Type {
	case "mysql":
		return EstimateMySQLSize(ctx, conn)
	case "postgres":
		return EstimatePostgreSQLSize(ctx, conn)
	case "redis":
		return EstimateRedisSize(ctx, conn)
	default:
		return 0, fmt.Errorf("unsupported database type: %s", conn.Type)
	}
}

// EstimateMySQLSize estimates MySQL database size
func EstimateMySQLSize(ctx context.Context, conn *config.Connection) (int64, error) {
	logger := util.GetLogger()

	// Build mysql command to query database size
	args := []string{
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--port=%d", conn.Port),
		fmt.Sprintf("--user=%s", conn.User),
	}

	if conn.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", conn.Password))
	}

	// Query to get database size
	query := fmt.Sprintf(
		"SELECT SUM(data_length + index_length) FROM information_schema.TABLES WHERE table_schema = '%s'",
		conn.Database,
	)
	args = append(args, "-N", "-B", "-e", query)

	// Execute query
	output, err := util.ExecuteCommandOutput(ctx, "mysql", args...)
	if err != nil {
		logger.DebugS("Failed to estimate MySQL size",
			"component", "progress",
			"database", conn.Database,
			"error", err)
		return 0, err
	}

	// Parse size
	sizeStr := strings.TrimSpace(string(output))
	if sizeStr == "" || sizeStr == "NULL" {
		return 0, nil // Empty database
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse database size: %w", err)
	}

	logger.DebugS("Estimated MySQL database size",
		"component", "progress",
		"database", conn.Database,
		"size_bytes", size)
	return size, nil
}

// EstimatePostgreSQLSize estimates PostgreSQL database size
func EstimatePostgreSQLSize(ctx context.Context, conn *config.Connection) (int64, error) {
	logger := util.GetLogger()

	// Build psql command to query database size
	args := []string{
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--port=%d", conn.Port),
		fmt.Sprintf("--username=%s", conn.User),
		"--no-password",
		"-t", // Tuples only (no headers)
		"-A", // Unaligned output
		"-c", // Command
		fmt.Sprintf("SELECT pg_database_size('%s')", conn.Database),
	}

	// Set PGPASSWORD environment variable
	env := map[string]string{}
	if conn.Password != "" {
		env["PGPASSWORD"] = conn.Password
	}

	// Execute query
	output, err := util.ExecuteCommandOutputWithEnv(ctx, env, "psql", args...)
	if err != nil {
		logger.DebugS("Failed to estimate PostgreSQL size",
			"component", "progress",
			"database", conn.Database,
			"error", err)
		return 0, err
	}

	// Parse size
	sizeStr := strings.TrimSpace(string(output))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse database size: %w", err)
	}

	logger.DebugS("Estimated PostgreSQL database size",
		"component", "progress",
		"database", conn.Database,
		"size_bytes", size)
	return size, nil
}

// EstimateRedisSize estimates Redis memory usage
func EstimateRedisSize(ctx context.Context, conn *config.Connection) (int64, error) {
	logger := util.GetLogger()

	// Build redis-cli command
	args := []string{
		"-h", conn.Host,
		"-p", fmt.Sprintf("%d", conn.Port),
	}

	if conn.Password != "" {
		args = append(args, "-a", conn.Password)
	}

	args = append(args, "INFO", "memory")

	// Execute command
	output, err := util.ExecuteCommandOutput(ctx, "redis-cli", args...)
	if err != nil {
		logger.DebugS("Failed to estimate Redis size",
			"component", "progress",
			"host", conn.Host,
			"port", conn.Port,
			"error", err)
		return 0, err
	}

	// Parse used_memory from output
	// Output format: "used_memory:12345\r\n..."
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "used_memory:") {
			sizeStr := strings.TrimPrefix(line, "used_memory:")
			sizeStr = strings.TrimSpace(sizeStr)
			size, err := strconv.ParseInt(sizeStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse Redis memory size: %w", err)
			}
			logger.DebugS("Estimated Redis size",
				"component", "progress",
				"host", conn.Host,
				"port", conn.Port,
				"size_bytes", size)
			return size, nil
		}
	}

	return 0, fmt.Errorf("could not find used_memory in Redis INFO output")
}
