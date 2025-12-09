package database

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
)

func TestNewPostgres(t *testing.T) {
	conn := fixtures.PostgresConnection()
	excludeTables := []string{"temp_table"}
	additionalArgs := []string{"--format=custom"}

	pg := NewPostgres(conn, excludeTables, additionalArgs)

	assert.NotNil(t, pg)
	assert.Equal(t, conn, pg.conn)
	assert.Equal(t, excludeTables, pg.excludeTables)
	assert.Equal(t, additionalArgs, pg.additionalArgs)
}

func TestPostgres_Name(t *testing.T) {
	tests := []struct {
		name     string
		conn     *config.Connection
		expected string
	}{
		{
			name: "standard connection",
			conn: &config.Connection{
				User:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
			},
			expected: "postgres:postgres@localhost:5432/testdb",
		},
		{
			name: "different user and port",
			conn: &config.Connection{
				User:     "dbadmin",
				Host:     "pg.example.com",
				Port:     5433,
				Database: "production",
			},
			expected: "postgres:dbadmin@pg.example.com:5433/production",
		},
		{
			name: "custom host",
			conn: &config.Connection{
				User:     "appuser",
				Host:     "192.168.1.50",
				Port:     5432,
				Database: "myapp",
			},
			expected: "postgres:appuser@192.168.1.50:5432/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPostgres(tt.conn, nil, nil)
			assert.Equal(t, tt.expected, pg.Name())
		})
	}
}

func TestPostgres_BuildDumpArgs(t *testing.T) {
	tests := []struct {
		name           string
		conn           *config.Connection
		excludeTables  []string
		additionalArgs []string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "basic connection",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			wantContains: []string{
				"--host=localhost",
				"--port=5432",
				"--username=postgres",
				"--no-password",
				"testdb",
			},
		},
		{
			name: "with password - uses PGPASSWORD env var",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "secret123",
				Database: "testdb",
			},
			wantContains: []string{
				"--no-password", // Password passed via PGPASSWORD env var
			},
			wantNotContain: []string{
				"--password=", // Should not be in args
				"secret123",   // Password should not be in args
			},
		},
		{
			name: "with excluded tables",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			excludeTables: []string{"temp_table", "cache_table"},
			wantContains: []string{
				"--exclude-table=temp_table",
				"--exclude-table=cache_table",
			},
		},
		{
			name: "with additional args",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			additionalArgs: []string{"--format=custom", "--compress=9"},
			wantContains: []string{
				"--format=custom",
				"--compress=9",
			},
		},
		{
			name: "with all options",
			conn: &config.Connection{
				Host:     "pg.example.com",
				Port:     5433,
				User:     "backup_user",
				Password: "backup_pass",
				Database: "production",
			},
			excludeTables:  []string{"sessions"},
			additionalArgs: []string{"--format=custom", "--verbose"},
			wantContains: []string{
				"--host=pg.example.com",
				"--port=5433",
				"--username=backup_user",
				"--no-password",
				"--exclude-table=sessions",
				"--format=custom",
				"--verbose",
				"production",
			},
		},
		{
			name: "custom port",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5433,
				User:     "postgres",
				Database: "testdb",
			},
			wantContains: []string{"--port=5433"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPostgres(tt.conn, tt.excludeTables, tt.additionalArgs)
			args := pg.buildDumpArgs()

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

func TestPostgres_BuildRestoreArgs(t *testing.T) {
	tests := []struct {
		name         string
		conn         *config.Connection
		wantContains []string
	}{
		{
			name: "basic restore args",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			wantContains: []string{
				"--host=localhost",
				"--port=5432",
				"--username=postgres",
				"--no-password",
				"testdb",
			},
		},
		{
			name: "restore with password via env var",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "secret",
				Database: "testdb",
			},
			wantContains: []string{
				"--no-password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPostgres(tt.conn, nil, nil)
			args := pg.buildRestoreArgs()

			argsStr := strings.Join(args, " ")
			for _, want := range tt.wantContains {
				assert.Contains(t, argsStr, want)
			}

			// Database name should be last
			assert.Equal(t, tt.conn.Database, args[len(args)-1])
		})
	}
}

func TestPostgres_Validate(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "validation check",
			wantErr: false, // Will depend on whether pg_dump is installed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPostgres(fixtures.PostgresConnection(), nil, nil)
			err := pg.Validate(context.Background())

			// We can't guarantee pg_dump is installed, so just check that
			// the function runs without panic and returns appropriate error
			if err != nil {
				assert.Contains(t, err.Error(), "pg_dump")
			}
		})
	}
}

func TestPostgres_Validate_ContextCancellation(_ *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pg := NewPostgres(fixtures.PostgresConnection(), nil, nil)

	// Should still work - CheckCommandExists doesn't use context
	// but we test to ensure no panic
	_ = pg.Validate(ctx)
}

func TestPostgres_Dump_Integration(t *testing.T) {
	tests := []struct {
		name    string
		conn    *config.Connection
		setup   func(*testing.T) context.Context
		wantErr bool
	}{
		{
			name:    "basic dump call structure",
			conn:    fixtures.PostgresConnection(),
			setup:   func(_ *testing.T) context.Context { return context.Background() },
			wantErr: true, // Will fail because pg_dump might not connect
		},
		{
			name: "dump with context cancellation",
			conn: fixtures.PostgresConnection(),
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
			pg := NewPostgres(tt.conn, nil, nil)
			ctx := tt.setup(t)
			var buf bytes.Buffer

			metadata, err := pg.Dump(ctx, &buf)

			if tt.wantErr {
				// We expect error because we're not actually connecting to Postgres
				assert.Error(t, err)
			} else {
				if err == nil {
					assert.NotNil(t, metadata)
					assert.Equal(t, "postgres", metadata.DatabaseType)
					assert.Equal(t, tt.conn.Database, metadata.DatabaseName)
					assert.Greater(t, metadata.Duration, time.Duration(0))
				}
			}
		})
	}
}

func TestPostgres_Restore_Integration(t *testing.T) {
	tests := []struct {
		name    string
		conn    *config.Connection
		input   string
		wantErr bool
	}{
		{
			name:    "basic restore call structure",
			conn:    fixtures.PostgresConnection(),
			input:   fixtures.MockPostgresDumpData(),
			wantErr: true, // Will fail without actual Postgres
		},
		{
			name:    "restore with empty input",
			conn:    fixtures.PostgresConnection(),
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPostgres(tt.conn, nil, nil)
			reader := strings.NewReader(tt.input)

			err := pg.Restore(context.Background(), reader)

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestPostgres_Dump_MetadataGeneration(t *testing.T) {
	pg := NewPostgres(fixtures.PostgresConnection(), nil, nil)

	// This will likely fail, but we're testing the metadata structure
	var buf bytes.Buffer
	metadata, err := pg.Dump(context.Background(), &buf)

	if err == nil {
		// If by chance pg_dump is available and works
		require.NotNil(t, metadata)
		assert.Equal(t, "postgres", metadata.DatabaseType)
		assert.Equal(t, "testdb", metadata.DatabaseName)
		assert.NotZero(t, metadata.Timestamp)
		assert.Greater(t, metadata.Duration, time.Duration(0))
	}
}

func TestPostgres_ExcludeMultipleTables(t *testing.T) {
	conn := fixtures.PostgresConnection()
	excludeTables := []string{"table1", "table2", "table3"}

	pg := NewPostgres(conn, excludeTables, nil)
	args := pg.buildDumpArgs()

	argsStr := strings.Join(args, " ")

	assert.Contains(t, argsStr, "--exclude-table=table1")
	assert.Contains(t, argsStr, "--exclude-table=table2")
	assert.Contains(t, argsStr, "--exclude-table=table3")
}

func TestPostgres_AdditionalArgsPreserved(t *testing.T) {
	conn := fixtures.PostgresConnection()
	additionalArgs := []string{
		"--format=custom",
		"--compress=9",
		"--verbose",
		"--blobs",
	}

	pg := NewPostgres(conn, nil, additionalArgs)
	args := pg.buildDumpArgs()

	argsStr := strings.Join(args, " ")

	for _, arg := range additionalArgs {
		assert.Contains(t, argsStr, arg)
	}
}

func TestPostgres_NoPasswordFlag(t *testing.T) {
	// Postgres always uses --no-password flag
	// Password is passed via PGPASSWORD environment variable
	conn := fixtures.PostgresConnection()

	pg := NewPostgres(conn, nil, nil)
	args := pg.buildDumpArgs()

	argsStr := strings.Join(args, " ")
	assert.Contains(t, argsStr, "--no-password")
	assert.NotContains(t, argsStr, "--password=")
}

func TestPostgres_ArgumentOrder(t *testing.T) {
	// Test that arguments are in correct order
	conn := &config.Connection{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Database: "testdb",
	}
	excludeTables := []string{"temp"}
	additionalArgs := []string{"--format=custom"}

	pg := NewPostgres(conn, excludeTables, additionalArgs)
	args := pg.buildDumpArgs()

	// Database name should always be last
	assert.Equal(t, "testdb", args[len(args)-1])

	// Basic connection args should come first
	assert.Contains(t, args[0], "--host=")
	assert.Contains(t, args[1], "--port=")
	assert.Contains(t, args[2], "--username=")
	assert.Equal(t, "--no-password", args[3])
}

func TestPostgres_UsernameVsUser(t *testing.T) {
	// Postgres uses --username, not --user
	conn := fixtures.PostgresConnection()

	pg := NewPostgres(conn, nil, nil)
	args := pg.buildDumpArgs()

	argsStr := strings.Join(args, " ")
	assert.Contains(t, argsStr, "--username=")
	assert.NotContains(t, argsStr, "--user=")
}
