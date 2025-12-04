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
func (p *Postgres) Validate(ctx context.Context) error {
	if err := util.CheckCommandExists("pg_dump"); err != nil {
		return fmt.Errorf("pg_dump not found: %w (install postgresql-client package)", err)
	}
	return nil
}

// Dump executes pg_dump and writes to the writer
func (p *Postgres) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	startTime := time.Now()

	args := p.buildDumpArgs()

	env := make(map[string]string)
	if p.conn.Password != "" {
		env["PGPASSWORD"] = p.conn.Password
	}

	var err error
	if len(env) > 0 {
		err = util.ExecuteCommandWithEnv(ctx, w, env, "pg_dump", args...)
	} else {
		err = util.ExecuteCommand(ctx, w, "pg_dump", args...)
	}

	if err != nil {
		return nil, fmt.Errorf("pg_dump failed: %w", err)
	}

	metadata := &DumpMetadata{
		DatabaseName: p.conn.Database,
		DatabaseType: "postgres",
		Duration:     time.Since(startTime),
		Timestamp:    startTime,
	}

	return metadata, nil
}

// Restore executes psql restore from the reader
func (p *Postgres) Restore(ctx context.Context, r io.Reader) error {
	args := p.buildRestoreArgs()

	env := make(map[string]string)
	if p.conn.Password != "" {
		env["PGPASSWORD"] = p.conn.Password
	}

	var err error
	if len(env) > 0 {
		err = util.ExecuteCommandWithStdinAndEnv(ctx, r, env, "psql", args...)
	} else {
		err = util.ExecuteCommandWithStdin(ctx, r, "psql", args...)
	}

	if err != nil {
		return fmt.Errorf("psql restore failed: %w", err)
	}

	return nil
}

func (p *Postgres) buildDumpArgs() []string {
	args := []string{
		fmt.Sprintf("--host=%s", p.conn.Host),
		fmt.Sprintf("--port=%d", p.conn.Port),
		fmt.Sprintf("--username=%s", p.conn.User),
		"--no-password", // Use PGPASSWORD env var instead
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
		"--no-password", // Use PGPASSWORD env var instead
		p.conn.Database,
	}

	return args
}

// ValidateConnection tests PostgreSQL connectivity
func (p *Postgres) ValidateConnection(ctx context.Context) error {
	// Check if psql command exists
	if err := util.CheckCommandExists("psql"); err != nil {
		return fmt.Errorf("psql not found: %w (install postgresql-client package)", err)
	}

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
	if p.conn.Password != "" {
		env["PGPASSWORD"] = p.conn.Password
	}

	// Execute test connection (use Discard since we just need to check connection)
	var err error
	if len(env) > 0 {
		err = util.ExecuteCommandWithEnv(ctx, io.Discard, env, "psql", args...)
	} else {
		err = util.ExecuteCommand(ctx, io.Discard, "psql", args...)
	}

	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	return nil
}
