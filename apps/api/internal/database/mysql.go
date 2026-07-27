package database

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/util"
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
	logger := util.GetLogger()

	// Prefer mariadb-dump, fall back to mysqldump
	cmd, err := util.DetectCommand("mariadb-dump", "mysqldump")
	if err != nil {
		logger.ErrorS("Failed to detect MySQL dump command",
			"component", "mysql",
			"database", m.conn.Database,
			"tried", []string{"mariadb-dump", "mysqldump"},
			"error", err)
		return fmt.Errorf("no dump command found: tried mariadb-dump, mysqldump (install mysql-client or mariadb-client package)")
	}
	m.dumpCmd = cmd

	logger.InfoS("MySQL dump command detected",
		"component", "mysql",
		"database", m.conn.Database,
		"command", cmd)

	return nil
}

// Dump executes mysqldump and writes to the writer
func (m *MySQL) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
	logger := util.GetLogger()
	startTime := time.Now()

	// Ensure command is detected (defensive programming)
	if m.dumpCmd == "" {
		if err := m.Validate(ctx); err != nil {
			return nil, err
		}
	}

	args := m.buildDumpArgs()
	sanitizedArgs := m.sanitizeArgs(args)

	logger.InfoS("Starting MySQL dump",
		"component", "mysql",
		"database", m.conn.Database,
		"host", m.conn.Host,
		"port", m.conn.Port,
		"command", m.dumpCmd,
		"args", sanitizedArgs)

	if err := util.ExecuteCommand(ctx, w, m.dumpCmd, args...); err != nil {
		logger.ErrorS("MySQL dump failed",
			"component", "mysql",
			"database", m.conn.Database,
			"command", m.dumpCmd,
			"duration", time.Since(startTime).String(),
			"error", err)
		return nil, fmt.Errorf("%s failed: %w", m.dumpCmd, err)
	}

	duration := time.Since(startTime)
	logger.InfoS("MySQL dump completed successfully",
		"component", "mysql",
		"database", m.conn.Database,
		"command", m.dumpCmd,
		"duration", duration.String())

	metadata := &DumpMetadata{
		DatabaseName: m.conn.Database,
		DatabaseType: "mysql",
		Duration:     duration,
		Timestamp:    startTime,
	}

	return metadata, nil
}

// Restore executes mysql restore from the reader
func (m *MySQL) Restore(ctx context.Context, r io.Reader) error {
	logger := util.GetLogger()
	startTime := time.Now()

	// Ensure command is detected (defensive programming)
	if m.restoreCmd == "" {
		if err := m.ValidateConnection(ctx); err != nil {
			return err
		}
	}

	args := m.buildRestoreArgs()
	sanitizedArgs := m.sanitizeArgs(args)

	logger.InfoS("Starting MySQL restore",
		"component", "mysql",
		"database", m.conn.Database,
		"host", m.conn.Host,
		"port", m.conn.Port,
		"command", m.restoreCmd,
		"args", sanitizedArgs,
		"binary_mode", "enabled")

	if err := util.ExecuteCommandWithStdin(ctx, r, m.restoreCmd, args...); err != nil {
		logger.ErrorS("MySQL restore failed",
			"component", "mysql",
			"database", m.conn.Database,
			"command", m.restoreCmd,
			"duration", time.Since(startTime).String(),
			"error", err)
		return fmt.Errorf("%s restore failed: %w", m.restoreCmd, err)
	}

	duration := time.Since(startTime)
	logger.InfoS("MySQL restore completed successfully",
		"component", "mysql",
		"database", m.conn.Database,
		"command", m.restoreCmd,
		"duration", duration.String())

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
	logger := util.GetLogger()

	// Prefer mariadb, fall back to mysql
	cmd, err := util.DetectCommand("mariadb", "mysql")
	if err != nil {
		logger.ErrorS("Failed to detect MySQL restore command",
			"component", "mysql",
			"database", m.conn.Database,
			"tried", []string{"mariadb", "mysql"},
			"error", err)
		return fmt.Errorf("no restore command found: tried mariadb, mysql (install mysql-client or mariadb-client package)")
	}
	m.restoreCmd = cmd

	logger.InfoS("MySQL restore command detected",
		"component", "mysql",
		"database", m.conn.Database,
		"command", cmd)

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

	logger.InfoS("Testing MySQL connection",
		"component", "mysql",
		"database", m.conn.Database,
		"host", m.conn.Host,
		"port", m.conn.Port,
		"command", cmd)

	if err := util.ExecuteCommand(ctx, io.Discard, cmd, args...); err != nil {
		logger.ErrorS("MySQL connection test failed",
			"component", "mysql",
			"database", m.conn.Database,
			"host", m.conn.Host,
			"port", m.conn.Port,
			"error", err)
		return fmt.Errorf("database connection failed: %w", err)
	}

	logger.InfoS("MySQL connection test successful",
		"component", "mysql",
		"database", m.conn.Database,
		"host", m.conn.Host,
		"port", m.conn.Port)

	return nil
}

// sanitizeArgs removes sensitive information from command arguments for
// logging. The rule lives in util.RedactArgs so the same masking applies to the
// error paths in util.ExecuteCommand — see issue #133.
func (m *MySQL) sanitizeArgs(args []string) []string {
	return util.RedactArgs(args)
}
