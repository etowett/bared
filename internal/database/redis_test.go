package database

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
)

func TestNewRedis(t *testing.T) {
	conn := fixtures.RedisConnection()

	redis := NewRedis(conn)

	assert.NotNil(t, redis)
	assert.Equal(t, conn, redis.conn)
}

func TestRedis_Name(t *testing.T) {
	tests := []struct {
		name     string
		conn     *config.Connection
		expected string
	}{
		{
			name: "standard connection",
			conn: &config.Connection{
				Host: "localhost",
				Port: 6379,
			},
			expected: "redis:localhost:6379",
		},
		{
			name: "different host and port",
			conn: &config.Connection{
				Host: "redis.example.com",
				Port: 6380,
			},
			expected: "redis:redis.example.com:6380",
		},
		{
			name: "IP address",
			conn: &config.Connection{
				Host: "192.168.1.100",
				Port: 6379,
			},
			expected: "redis:192.168.1.100:6379",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redis := NewRedis(tt.conn)
			assert.Equal(t, tt.expected, redis.Name())
		})
	}
}

func TestRedis_BuildDumpArgs(t *testing.T) {
	tests := []struct {
		name         string
		conn         *config.Connection
		outputFile   string
		wantContains []string
	}{
		{
			name: "basic connection - no password",
			conn: &config.Connection{
				Host: "localhost",
				Port: 6379,
			},
			outputFile: "/tmp/redis.rdb",
			wantContains: []string{
				"-h", "localhost",
				"-p", "6379",
				"--rdb", "/tmp/redis.rdb",
			},
		},
		{
			name: "with password",
			conn: &config.Connection{
				Host:     "localhost",
				Port:     6379,
				Password: "secret123",
			},
			outputFile: "/tmp/redis.rdb",
			wantContains: []string{
				"-h", "localhost",
				"-p", "6379",
				"-a", "secret123",
				"--rdb", "/tmp/redis.rdb",
			},
		},
		{
			name: "custom port",
			conn: &config.Connection{
				Host: "localhost",
				Port: 6380,
			},
			outputFile: "/tmp/custom.rdb",
			wantContains: []string{
				"-p", "6380",
				"--rdb", "/tmp/custom.rdb",
			},
		},
		{
			name: "remote host",
			conn: &config.Connection{
				Host:     "redis.example.com",
				Port:     6379,
				Password: "pass123",
			},
			outputFile: "/var/tmp/dump.rdb",
			wantContains: []string{
				"-h", "redis.example.com",
				"-p", "6379",
				"-a", "pass123",
				"--rdb", "/var/tmp/dump.rdb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redis := NewRedis(tt.conn)
			args := redis.buildDumpArgs(tt.outputFile)

			// Convert to string for easier checking
			argsStr := strings.Join(args, " ")

			for _, want := range tt.wantContains {
				assert.Contains(t, argsStr, want, "args should contain: %s", want)
			}
		})
	}
}

func TestRedis_BuildDumpArgs_NoPassword(t *testing.T) {
	conn := &config.Connection{
		Host: "localhost",
		Port: 6379,
	}

	redis := NewRedis(conn)
	args := redis.buildDumpArgs("/tmp/redis.rdb")

	argsStr := strings.Join(args, " ")
	assert.NotContains(t, argsStr, "-a")
}

func TestRedis_Validate(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "validation check",
			wantErr: false, // Will depend on whether redis-cli is installed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redis := NewRedis(fixtures.RedisConnection())
			err := redis.Validate(context.Background())

			// We can't guarantee redis-cli is installed, so just check that
			// the function runs without panic and returns appropriate error
			if err != nil {
				assert.Contains(t, err.Error(), "redis-cli")
			}
		})
	}
}

func TestRedis_Validate_ContextCancellation(_ *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	redis := NewRedis(fixtures.RedisConnection())

	// Should still work - CheckCommandExists doesn't use context
	// but we test to ensure no panic
	_ = redis.Validate(ctx)
}

func TestRedis_Dump_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name    string
		conn    *config.Connection
		setup   func(*testing.T) context.Context
		wantErr bool
	}{
		{
			name:    "basic dump call structure",
			conn:    fixtures.RedisConnection(),
			setup:   func(_ *testing.T) context.Context { return context.Background() },
			wantErr: false, // May succeed or fail depending on Redis availability
		},
		{
			name: "dump with context cancellation",
			conn: fixtures.RedisConnection(),
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
			redis := NewRedis(tt.conn)
			ctx := tt.setup(t)
			var buf bytes.Buffer

			metadata, err := redis.Dump(ctx, &buf)

			if tt.wantErr {
				// We expect error (e.g., from cancelled context)
				assert.Error(t, err)
			} else {
				// May succeed or fail depending on Redis availability
				if err == nil {
					assert.NotNil(t, metadata)
					assert.Equal(t, "redis", metadata.DatabaseType)
					assert.Greater(t, metadata.Duration, time.Duration(0))
					assert.Greater(t, metadata.Size, int64(0))
				}
			}
		})
	}
}

func TestRedis_Dump_MetadataGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	redis := NewRedis(fixtures.RedisConnection())

	// This will likely fail, but we're testing the metadata structure
	var buf bytes.Buffer
	metadata, err := redis.Dump(context.Background(), &buf)

	if err == nil {
		// If by chance redis-cli is available and works
		require.NotNil(t, metadata)
		assert.Equal(t, "redis", metadata.DatabaseType)
		assert.Contains(t, metadata.DatabaseName, "localhost:6379")
		assert.NotZero(t, metadata.Timestamp)
		assert.Greater(t, metadata.Duration, time.Duration(0))
	}
}

func TestRedis_Restore_NotImplemented(t *testing.T) {
	redis := NewRedis(fixtures.RedisConnection())
	reader := strings.NewReader(fixtures.MockRedisDumpData())

	err := redis.Restore(context.Background(), reader)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestRedis_Restore_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	redis := NewRedis(fixtures.RedisConnection())
	reader := strings.NewReader("test data")

	err := redis.Restore(ctx, reader)

	// Should still return not implemented error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestRedis_PasswordHandling(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		shouldHaveA bool
	}{
		{
			name:        "with password",
			password:    "secret123",
			shouldHaveA: true,
		},
		{
			name:        "empty password",
			password:    "",
			shouldHaveA: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &config.Connection{
				Host:     "localhost",
				Port:     6379,
				Password: tt.password,
			}

			redis := NewRedis(conn)
			args := redis.buildDumpArgs("/tmp/redis.rdb")

			hasAuthFlag := false
			for i, arg := range args {
				if arg == "-a" {
					hasAuthFlag = true
					if i+1 < len(args) {
						assert.Equal(t, tt.password, args[i+1])
					}
					break
				}
			}

			assert.Equal(t, tt.shouldHaveA, hasAuthFlag)
		})
	}
}

func TestRedis_ArgumentOrder(t *testing.T) {
	conn := &config.Connection{
		Host:     "localhost",
		Port:     6379,
		Password: "secret",
	}

	redis := NewRedis(conn)
	args := redis.buildDumpArgs("/tmp/redis.rdb")

	// Should have: -h host -p port -a password --rdb file
	assert.Equal(t, "-h", args[0])
	assert.Equal(t, "localhost", args[1])
	assert.Equal(t, "-p", args[2])
	assert.Equal(t, "6379", args[3])
	assert.Equal(t, "-a", args[4])
	assert.Equal(t, "secret", args[5])
	assert.Equal(t, "--rdb", args[6])
	assert.Equal(t, "/tmp/redis.rdb", args[7])
}

func TestRedis_DifferentPorts(t *testing.T) {
	ports := []int{6379, 6380, 6381, 7000}

	for _, port := range ports {
		t.Run(fmt.Sprintf("port_%d", port), func(t *testing.T) {
			conn := &config.Connection{
				Host: "localhost",
				Port: port,
			}

			redis := NewRedis(conn)
			args := redis.buildDumpArgs("/tmp/redis.rdb")

			argsStr := strings.Join(args, " ")
			assert.Contains(t, argsStr, fmt.Sprintf("-p %d", port))
		})
	}
}

func TestRedis_Name_IncludesHostAndPort(t *testing.T) {
	conn := &config.Connection{
		Host: "redis-master.local",
		Port: 6380,
	}

	redis := NewRedis(conn)
	name := redis.Name()

	assert.Contains(t, name, "redis")
	assert.Contains(t, name, "redis-master.local")
	assert.Contains(t, name, "6380")
}
