package database

import (
	"context"
	"fmt"
	"io"
	"time"

	"bared/internal/config"
	"bared/internal/util"
)

// MySQL implements Dumper and Restorer for MySQL/MariaDB databases
type MySQL struct {
	conn           *config.Connection
	excludeTables  []string
	additionalArgs []string
}

// NewMySQL creates a new MySQL dumper
func NewMySQL(conn *config.Connection, excludeTables []string, additionalArgs []string) *MySQL {
	return &MySQL{
		conn:           conn,
		excludeTables:  excludeTables,
		additionalArgs: additionalArgs,
	}
}

// Name returns the database name
func (m *MySQL) Name() string {
	return fmt.Sprintf("mysql:%s@%s:%d/%s", m.conn.User, m.conn.Host, m.conn.Port, m.conn.Database)
}

// Validate checks if mysqldump command exists
func (m *MySQL) Validate(ctx context.Context) error {
	if err := util.CheckCommandExists("mysqldump"); err != nil {
		return fmt.Errorf("mysqldump not found: %w (install mysql-client package)", err)
	}
	return nil
}

// Dump executes mysqldump and writes to the writer
func (m *MySQL) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	startTime := time.Now()

	args := m.buildDumpArgs()

	if err := util.ExecuteCommand(ctx, w, "mysqldump", args...); err != nil {
		return nil, fmt.Errorf("mysqldump failed: %w", err)
	}

	metadata := &DumpMetadata{
		DatabaseName: m.conn.Database,
		DatabaseType: "mysql",
		Duration:     time.Since(startTime),
		Timestamp:    startTime,
	}

	return metadata, nil
}

// Restore executes mysql restore from the reader
func (m *MySQL) Restore(ctx context.Context, r io.Reader) error {
	args := m.buildRestoreArgs()

	if err := util.ExecuteCommandWithStdin(ctx, r, "mysql", args...); err != nil {
		return fmt.Errorf("mysql restore failed: %w", err)
	}

	return nil
}

func (m *MySQL) buildDumpArgs() []string {
	args := []string{
		fmt.Sprintf("--host=%s", m.conn.Host),
		fmt.Sprintf("--port=%d", m.conn.Port),
		fmt.Sprintf("--user=%s", m.conn.User),
	}

	if m.conn.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", m.conn.Password))
	}

	// Add exclude tables
	for _, table := range m.excludeTables {
		args = append(args, fmt.Sprintf("--ignore-table=%s.%s", m.conn.Database, table))
	}

	// Add additional arguments
	args = append(args, m.additionalArgs...)

	// Add database name last
	args = append(args, m.conn.Database)

	return args
}

func (m *MySQL) buildRestoreArgs() []string {
	args := []string{
		fmt.Sprintf("--host=%s", m.conn.Host),
		fmt.Sprintf("--port=%d", m.conn.Port),
		fmt.Sprintf("--user=%s", m.conn.User),
	}

	if m.conn.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", m.conn.Password))
	}

	// Add database name
	args = append(args, m.conn.Database)

	return args
}
