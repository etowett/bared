package database

import (
	"context"
	"fmt"
	"io"
	"time"

	"bared/internal/config"
	"bared/internal/util"
)

// Postgres implements Dumper and Restorer for PostgreSQL databases
type Postgres struct {
	conn           *config.Connection
	excludeTables  []string
	additionalArgs []string
}

// NewPostgres creates a new PostgreSQL dumper
func NewPostgres(conn *config.Connection, excludeTables []string, additionalArgs []string) *Postgres {
	return &Postgres{
		conn:           conn,
		excludeTables:  excludeTables,
		additionalArgs: additionalArgs,
	}
}

// Name returns the database name
func (p *Postgres) Name() string {
	return fmt.Sprintf("postgres:%s@%s:%d/%s", p.conn.User, p.conn.Host, p.conn.Port, p.conn.Database)
}

// Validate checks if pg_dump command exists
func (p *Postgres) Validate(_ context.Context) error {
	logger := util.GetLogger()

	if err := util.CheckCommandExists("pg_dump"); err != nil {
		logger.ErrorS("Failed to detect PostgreSQL dump command",
			"component", "postgres",
			"database", p.conn.Database,
			"command", "pg_dump",
			"error", err)
		return fmt.Errorf("pg_dump not found: %w (install postgresql-client package)", err)
	}

	logger.InfoS("PostgreSQL dump command detected",
		"component", "postgres",
		"database", p.conn.Database,
		"command", "pg_dump")

	return nil
}

// Dump executes pg_dump and writes to the writer
func (p *Postgres) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	logger := util.GetLogger()
	startTime := time.Now()

	args := p.buildDumpArgs()

	env := make(map[string]string)
	passwordSet := false
	if p.conn.Password != "" {
		env["PGPASSWORD"] = p.conn.Password
		passwordSet = true
	}

	logger.InfoS("Starting PostgreSQL dump",
		"component", "postgres",
		"database", p.conn.Database,
		"host", p.conn.Host,
		"port", p.conn.Port,
		"command", "pg_dump",
		"password_env", passwordSet,
		"args", args)

	var err error
	if len(env) > 0 {
		err = util.ExecuteCommandWithEnv(ctx, w, env, "pg_dump", args...)
	} else {
		err = util.ExecuteCommand(ctx, w, "pg_dump", args...)
	}

	if err != nil {
		logger.ErrorS("PostgreSQL dump failed",
			"component", "postgres",
			"database", p.conn.Database,
			"command", "pg_dump",
			"duration", time.Since(startTime).String(),
			"error", err)
		return nil, fmt.Errorf("pg_dump failed: %w", err)
	}

	duration := time.Since(startTime)
	logger.InfoS("PostgreSQL dump completed successfully",
		"component", "postgres",
		"database", p.conn.Database,
		"command", "pg_dump",
		"duration", duration.String())

	metadata := &DumpMetadata{
		DatabaseName: p.conn.Database,
		DatabaseType: "postgres",
		Duration:     duration,
		Timestamp:    startTime,
	}

	return metadata, nil
}

// Restore executes psql restore from the reader
func (p *Postgres) Restore(ctx context.Context, r io.Reader) error {
	logger := util.GetLogger()
	startTime := time.Now()

	args := p.buildRestoreArgs()

	env := make(map[string]string)
	passwordSet := false
	if p.conn.Password != "" {
		env["PGPASSWORD"] = p.conn.Password
		passwordSet = true
	}

	logger.InfoS("Starting PostgreSQL restore",
		"component", "postgres",
		"database", p.conn.Database,
		"host", p.conn.Host,
		"port", p.conn.Port,
		"command", "psql",
		"password_env", passwordSet,
		"args", args)

	var err error
	if len(env) > 0 {
		err = util.ExecuteCommandWithStdinAndEnv(ctx, r, env, "psql", args...)
	} else {
		err = util.ExecuteCommandWithStdin(ctx, r, "psql", args...)
	}

	if err != nil {
		logger.ErrorS("PostgreSQL restore failed",
			"component", "postgres",
			"database", p.conn.Database,
			"command", "psql",
			"duration", time.Since(startTime).String(),
			"error", err)
		return fmt.Errorf("psql restore failed: %w", err)
	}

	duration := time.Since(startTime)
	logger.InfoS("PostgreSQL restore completed successfully",
		"component", "postgres",
		"database", p.conn.Database,
		"command", "psql",
		"duration", duration.String())

	return nil
}

func (p *Postgres) buildDumpArgs() []string {
	args := []string{
		fmt.Sprintf("--host=%s", p.conn.Host),
		fmt.Sprintf("--port=%d", p.conn.Port),
		fmt.Sprintf("--username=%s", p.conn.User),
		"--no-password", // Use PGPASSWORD env var instead
		"--no-owner",    // Omit ownership commands for cross-user restores
		"--no-acl",      // Omit GRANT/REVOKE/ALTER DEFAULT PRIVILEGES (limited target roles can't apply them)
	}

	// Add exclude tables
	for _, table := range p.excludeTables {
		args = append(args, fmt.Sprintf("--exclude-table=%s", table))
	}

	// Add additional arguments
	args = append(args, p.additionalArgs...)

	// Add database name last
	args = append(args, p.conn.Database)

	return args
}

func (p *Postgres) buildRestoreArgs() []string {
	args := []string{
		fmt.Sprintf("--host=%s", p.conn.Host),
		fmt.Sprintf("--port=%d", p.conn.Port),
		fmt.Sprintf("--username=%s", p.conn.User),
		"--no-password",             // Use PGPASSWORD env var instead
		"--set", "ON_ERROR_STOP=on", // Stop on first error
		p.conn.Database,
	}

	return args
}

// ValidateConnection tests PostgreSQL connectivity
func (p *Postgres) ValidateConnection(ctx context.Context) error {
	logger := util.GetLogger()

	// Check if psql command exists
	if err := util.CheckCommandExists("psql"); err != nil {
		logger.ErrorS("Failed to detect PostgreSQL restore command",
			"component", "postgres",
			"database", p.conn.Database,
			"command", "psql",
			"error", err)
		return fmt.Errorf("psql not found: %w (install postgresql-client package)", err)
	}

	logger.InfoS("PostgreSQL restore command detected",
		"component", "postgres",
		"database", p.conn.Database,
		"command", "psql")

	// Build test connection args
	args := []string{
		fmt.Sprintf("--host=%s", p.conn.Host),
		fmt.Sprintf("--port=%d", p.conn.Port),
		fmt.Sprintf("--username=%s", p.conn.User),
		"--no-password",
		p.conn.Database,
		"-c", "SELECT 1;", // Simple test query
	}

	env := make(map[string]string)
	passwordSet := false
	if p.conn.Password != "" {
		env["PGPASSWORD"] = p.conn.Password
		passwordSet = true
	}

	logger.InfoS("Testing PostgreSQL connection",
		"component", "postgres",
		"database", p.conn.Database,
		"host", p.conn.Host,
		"port", p.conn.Port,
		"command", "psql",
		"password_env", passwordSet)

	// Execute test connection (use Discard since we just need to check connection)
	var err error
	if len(env) > 0 {
		err = util.ExecuteCommandWithEnv(ctx, io.Discard, env, "psql", args...)
	} else {
		err = util.ExecuteCommand(ctx, io.Discard, "psql", args...)
	}

	if err != nil {
		logger.ErrorS("PostgreSQL connection test failed",
			"component", "postgres",
			"database", p.conn.Database,
			"host", p.conn.Host,
			"port", p.conn.Port,
			"error", err)
		return fmt.Errorf("database connection failed: %w", err)
	}

	logger.InfoS("PostgreSQL connection test successful",
		"component", "postgres",
		"database", p.conn.Database,
		"host", p.conn.Host,
		"port", p.conn.Port)

	return nil
}
