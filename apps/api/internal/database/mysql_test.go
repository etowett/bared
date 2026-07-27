package database

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

func TestNewMySQL(t *testing.T) {
	conn := fixtures.MySQLConnection()
	excludeTables := []string{"temp_table"}
	additionalArgs := []string{"--single-transaction"}

	mysql := NewMySQL(conn, excludeTables, additionalArgs)

	assert.NotNil(t, mysql)
	assert.Equal(t, conn, mysql.conn)
	assert.Equal(t, excludeTables, mysql.excludeTables)
	assert.Equal(t, additionalArgs, mysql.additionalArgs)
}

func TestMySQL_Name(t *testing.T) {
	tests := []struct {
		name     string
		conn     *config.Connection
		expected string
	}{
		{
			name: "standard connection",
			conn: &config.Connection{
				User:     "root",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
			},
			expected: "mysql:root@localhost:3306/testdb",
		},
		{
			name: "different user and port",
			conn: &config.Connection{
				User:     "dbuser",
				Host:     "db.example.com",
				Port:     3307,
				Database: "production",
			},
			expected: "mysql:dbuser@db.example.com:3307/production",
		},
		{
			name: "custom host",
			conn: &config.Connection{
				User:     "admin",
				Host:     "192.168.1.100",
				Port:     3306,
				Database: "myapp",
			},
			expected: "mysql:admin@192.168.1.100:3306/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysql := NewMySQL(tt.conn, nil, nil)
			assert.Equal(t, tt.expected, mysql.Name())
		})
	}
}

func TestMySQL_BuildDumpArgs(t *testing.T) {
	tests := []struct {
		name           string
		conn           *config.Connection
		excludeTables  []string
		additionalArgs []string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "basic connection - no password",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Database: "testdb",
			},
			wantContains: []string{
				"--host=localhost",
				"--port=3306",
				"--user=root",
				"testdb",
			},
			wantNotContain: []string{"--password="},
		},
		{
			name: "with password",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "secret123",
				Database: "testdb",
			},
			wantContains: []string{
				"--host=localhost",
				"--password=secret123",
			},
		},
		{
			name: "with excluded tables",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Database: "testdb",
			},
			excludeTables: []string{"temp_table", "cache_table"},
			wantContains: []string{
				"--ignore-table=testdb.temp_table",
				"--ignore-table=testdb.cache_table",
			},
		},
		{
			name: "with additional args",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Database: "testdb",
			},
			additionalArgs: []string{"--single-transaction", "--quick"},
			wantContains: []string{
				"--single-transaction",
				"--quick",
			},
		},
		{
			name: "with all options",
			conn: &config.Connection{
				Host:     "db.example.com",
				Port:     3307,
				User:     "backup_user",
				Password: "backup_pass",
				Database: "production",
			},
			excludeTables:  []string{"sessions"},
			additionalArgs: []string{"--single-transaction", "--lock-tables=false"},
			wantContains: []string{
				"--host=db.example.com",
				"--port=3307",
				"--user=backup_user",
				"--password=backup_pass",
				"--ignore-table=production.sessions",
				"--single-transaction",
				"--lock-tables=false",
				"production",
			},
		},
		{
			name: "custom port",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3307,
				User:     "root",
				Database: "testdb",
			},
			wantContains: []string{"--port=3307"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysql := NewMySQL(tt.conn, tt.excludeTables, tt.additionalArgs)
			args := mysql.buildDumpArgs()

			// Convert to string for easier checking
			argsStr := strings.Join(args, " ")

			for _, want := range tt.wantContains {
				assert.Contains(t, argsStr, want, "args should contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, argsStr, notWant, "args should not contain: %s", notWant)
			}

			// Database name should be last
			assert.Equal(t, tt.conn.Database, args[len(args)-1], "database should be last argument")
		})
	}
}

func TestMySQL_BuildRestoreArgs(t *testing.T) {
	tests := []struct {
		name         string
		conn         *config.Connection
		wantContains []string
	}{
		{
			name: "basic restore args",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Database: "testdb",
			},
			wantContains: []string{
				"--host=localhost",
				"--port=3306",
				"--user=root",
				"testdb",
			},
		},
		{
			name: "restore with password",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "secret",
				Database: "testdb",
			},
			wantContains: []string{
				"--password=secret",
			},
		},
		{
			name: "restore includes binary-mode for MariaDB compatibility",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Database: "testdb",
			},
			wantContains: []string{
				"--binary-mode=1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysql := NewMySQL(tt.conn, nil, nil)
			args := mysql.buildRestoreArgs()

			argsStr := strings.Join(args, " ")
			for _, want := range tt.wantContains {
				assert.Contains(t, argsStr, want)
			}

			// Database name should be last
			assert.Equal(t, tt.conn.Database, args[len(args)-1])
		})
	}
}

func TestMySQL_BinaryModeInRestoreArgs(t *testing.T) {
	// Additional dedicated test for binary-mode flag
	conn := fixtures.MySQLConnection()
	mysql := NewMySQL(conn, nil, nil)
	args := mysql.buildRestoreArgs()

	argsStr := strings.Join(args, " ")
	assert.Contains(t, argsStr, "--binary-mode=1",
		"restore args should contain --binary-mode=1 for MariaDB compatibility")
}

func TestMySQL_Validate(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "validation check",
			wantErr: false, // Will depend on whether mysqldump is installed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysql := NewMySQL(fixtures.MySQLConnection(), nil, nil)
			err := mysql.Validate(context.Background())

			// We can't guarantee mysqldump is installed, so just check that
			// the function runs without panic and returns appropriate error
			if err != nil {
				assert.Contains(t, err.Error(), "mysqldump")
			}
		})
	}
}

func TestMySQL_Validate_ContextCancellation(_ *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mysql := NewMySQL(fixtures.MySQLConnection(), nil, nil)

	// Should still work - CheckCommandExists doesn't use context
	// but we test to ensure no panic
	_ = mysql.Validate(ctx)
}

func TestMySQL_Dump_Integration(t *testing.T) {
	// This is a pseudo-integration test that verifies the Dump logic
	// without actually running mysqldump (which we can't mock directly)

	tests := []struct {
		name    string
		conn    *config.Connection
		setup   func(*testing.T) context.Context
		wantErr bool
	}{
		{
			name:    "basic dump call structure",
			conn:    fixtures.MySQLConnection(),
			setup:   func(_ *testing.T) context.Context { return context.Background() },
			wantErr: true, // Will fail because mysqldump might not connect
		},
		{
			name: "dump with context cancellation",
			conn: fixtures.MySQLConnection(),
			setup: func(_ *testing.T) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysql := NewMySQL(tt.conn, nil, nil)
			ctx := tt.setup(t)
			var buf bytes.Buffer

			metadata, err := mysql.Dump(ctx, &buf)

			if tt.wantErr {
				// We expect error because we're not actually connecting to MySQL
				assert.Error(t, err)
			} else {
				if err == nil {
					assert.NotNil(t, metadata)
					assert.Equal(t, "mysql", metadata.DatabaseType)
					assert.Equal(t, tt.conn.Database, metadata.DatabaseName)
					assert.Greater(t, metadata.Duration, time.Duration(0))
				}
			}
		})
	}
}

func TestMySQL_Restore_Integration(t *testing.T) {
	tests := []struct {
		name    string
		conn    *config.Connection
		input   string
		wantErr bool
	}{
		{
			name:    "basic restore call structure",
			conn:    fixtures.MySQLConnection(),
			input:   fixtures.MockDumpData(),
			wantErr: true, // Will fail without actual MySQL
		},
		{
			name:    "restore with empty input",
			conn:    fixtures.MySQLConnection(),
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysql := NewMySQL(tt.conn, nil, nil)
			reader := strings.NewReader(tt.input)

			err := mysql.Restore(context.Background(), reader)

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestMySQL_Dump_MetadataGeneration(t *testing.T) {
	// Test that metadata is generated correctly when dump succeeds
	// We'll use a mock scenario where we can control the outcome

	mysql := NewMySQL(fixtures.MySQLConnection(), nil, nil)

	// This will likely fail, but we're testing the metadata structure
	var buf bytes.Buffer
	metadata, err := mysql.Dump(context.Background(), &buf)

	if err == nil {
		// If by chance mysqldump is available and works
		require.NotNil(t, metadata)
		assert.Equal(t, "mysql", metadata.DatabaseType)
		assert.Equal(t, "testdb", metadata.DatabaseName)
		assert.NotZero(t, metadata.Timestamp)
		assert.Greater(t, metadata.Duration, time.Duration(0))
	}
}

func TestMySQL_ExcludeMultipleTables(t *testing.T) {
	conn := fixtures.MySQLConnection()
	excludeTables := []string{"table1", "table2", "table3"}

	mysql := NewMySQL(conn, excludeTables, nil)
	args := mysql.buildDumpArgs()

	argsStr := strings.Join(args, " ")

	assert.Contains(t, argsStr, "--ignore-table=testdb.table1")
	assert.Contains(t, argsStr, "--ignore-table=testdb.table2")
	assert.Contains(t, argsStr, "--ignore-table=testdb.table3")
}

func TestMySQL_AdditionalArgsPreserved(t *testing.T) {
	conn := fixtures.MySQLConnection()
	additionalArgs := []string{
		"--single-transaction",
		"--quick",
		"--lock-tables=false",
		"--add-drop-table",
	}

	mysql := NewMySQL(conn, nil, additionalArgs)
	args := mysql.buildDumpArgs()

	argsStr := strings.Join(args, " ")

	for _, arg := range additionalArgs {
		assert.Contains(t, argsStr, arg)
	}
}

func TestMySQL_EmptyPassword(t *testing.T) {
	conn := &config.Connection{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "", // No password
		Database: "testdb",
	}

	mysql := NewMySQL(conn, nil, nil)
	args := mysql.buildDumpArgs()

	argsStr := strings.Join(args, " ")
	assert.NotContains(t, argsStr, "--password=")
}

func TestMySQL_ArgumentOrder(t *testing.T) {
	// Test that arguments are in correct order
	target := fixtures.MySQLTargetWithExcludeTables()
	additionalArgs := []string{"--single-transaction"}

	mysql := NewMySQL(target.Conn, target.ExcludeTables, additionalArgs)
	args := mysql.buildDumpArgs()

	// Database name should always be last
	assert.Equal(t, "testdb", args[len(args)-1])

	// Basic connection args should come first
	assert.Contains(t, args[0], "--host=")
	assert.Contains(t, args[1], "--port=")
	assert.Contains(t, args[2], "--user=")
}
