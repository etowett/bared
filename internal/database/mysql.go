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
	dumpCmd        string // cached: mariadb-dump or mysqldump
	restoreCmd     string // cached: mariadb or mysql
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
func (m *MySQL) Validate(_ context.Context) error {
	// Prefer mariadb-dump, fall back to mysqldump
	cmd, err := util.DetectCommand("mariadb-dump", "mysqldump")
	if err != nil {
		return fmt.Errorf("no dump command found: tried mariadb-dump, mysqldump (install mysql-client or mariadb-client package)")
	}
	m.dumpCmd = cmd
	return nil
}

// Dump executes mysqldump and writes to the writer
func (m *MySQL) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	startTime := time.Now()

	// Ensure command is detected (defensive programming)
	if m.dumpCmd == "" {
		if err := m.Validate(ctx); err != nil {
			return nil, err
		}
	}

	args := m.buildDumpArgs()

	if err := util.ExecuteCommand(ctx, w, m.dumpCmd, args...); err != nil {
		return nil, fmt.Errorf("%s failed: %w", m.dumpCmd, err)
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
	// Ensure command is detected (defensive programming)
	if m.restoreCmd == "" {
		if err := m.ValidateConnection(ctx); err != nil {
			return err
		}
	}

	args := m.buildRestoreArgs()

	if err := util.ExecuteCommandWithStdin(ctx, r, m.restoreCmd, args...); err != nil {
		return fmt.Errorf("%s restore failed: %w", m.restoreCmd, err)
	}

	return nil
}

func (m *MySQL) buildDumpArgs() []string {
	args := []string{
		fmt.Sprintf("--host=%s", m.conn.Host),
		fmt.Sprintf("--port=%d", m.conn.Port),
		fmt.Sprintf("--user=%s", m.conn.User),
		"--skip-ssl", // Disable SSL (compatible with both MySQL and MariaDB)
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
		"--skip-ssl",      // Disable SSL (compatible with both MySQL and MariaDB)
		"--binary-mode=1", // Required for MariaDB binary data handling
	}

	if m.conn.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", m.conn.Password))
	}

	// Add database name
	args = append(args, m.conn.Database)

	return args
}

// ValidateConnection tests MySQL connectivity
func (m *MySQL) ValidateConnection(ctx context.Context) error {
	// Prefer mariadb, fall back to mysql
	cmd, err := util.DetectCommand("mariadb", "mysql")
	if err != nil {
		return fmt.Errorf("no restore command found: tried mariadb, mysql (install mysql-client or mariadb-client package)")
	}
	m.restoreCmd = cmd

	// Build test connection args with binary-mode for MariaDB compatibility
	args := []string{
		fmt.Sprintf("--host=%s", m.conn.Host),
		fmt.Sprintf("--port=%d", m.conn.Port),
		fmt.Sprintf("--user=%s", m.conn.User),
		"--skip-ssl",      // Disable SSL (compatible with both MySQL and MariaDB)
		"--binary-mode=1", // Required for MariaDB binary data handling
	}

	if m.conn.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", m.conn.Password))
	}

	args = append(args, m.conn.Database, "-e", "SELECT 1;")

	if err := util.ExecuteCommand(ctx, io.Discard, cmd, args...); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	return nil
}
