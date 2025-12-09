package progress

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
)

// Note: These tests are limited because the estimator functions execute real database commands.
// Full integration testing would require actual database instances.
// These tests focus on routing logic, error handling, and API contracts.

func TestEstimateDatabaseSize_UnsupportedType(t *testing.T) {
	tests := []struct {
		name   string
		dbType string
	}{
		{
			name:   "unsupported mongodb",
			dbType: "mongodb",
		},
		{
			name:   "unsupported cassandra",
			dbType: "cassandra",
		},
		{
			name:   "unsupported elasticsearch",
			dbType: "elasticsearch",
		},
		{
			name:   "empty type",
			dbType: "",
		},
		{
			name:   "invalid type",
			dbType: "invalid-db-type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &config.Connection{
				Type:     tt.dbType,
				Host:     "localhost",
				Port:     5432,
				User:     "test",
				Password: "test",
				Database: "testdb",
			}

			ctx := context.Background()
			size, err := EstimateDatabaseSize(ctx, conn)

			require.Error(t, err)
			assert.Equal(t, int64(0), size)
			assert.Contains(t, err.Error(), "unsupported database type")
		})
	}
}

func TestEstimateDatabaseSize_ContextCancellation(t *testing.T) {
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tests := []struct {
		name   string
		dbType string
	}{
		{name: "mysql with cancelled context", dbType: "mysql"},
		{name: "postgres with cancelled context", dbType: "postgres"},
		{name: "redis with cancelled context", dbType: "redis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &config.Connection{
				Type:     tt.dbType,
				Host:     "localhost",
				Port:     3306,
				User:     "test",
				Password: "test",
				Database: "testdb",
			}

			// Should fail due to context cancellation (command won't execute)
			size, err := EstimateDatabaseSize(ctx, conn)

			// Error expected since context is cancelled
			assert.Error(t, err)
			assert.Equal(t, int64(0), size)
		})
	}
}

func TestEstimateMySQLSize_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		conn *config.Connection
	}{
		{
			name: "non-existent host",
			conn: &config.Connection{
				Type:     "mysql",
				Host:     "non-existent-host-12345.invalid",
				Port:     3306,
				User:     "test",
				Password: "test",
				Database: "testdb",
			},
		},
		{
			name: "invalid port",
			conn: &config.Connection{
				Type:     "mysql",
				Host:     "localhost",
				Port:     99999, // Invalid port
				User:     "test",
				Password: "test",
				Database: "testdb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := EstimateMySQLSize(ctx, tt.conn)

			// Should fail due to invalid connection
			assert.Error(t, err)
			assert.Equal(t, int64(0), size)
		})
	}
}

func TestEstimatePostgreSQLSize_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		conn *config.Connection
	}{
		{
			name: "non-existent host",
			conn: &config.Connection{
				Type:     "postgres",
				Host:     "non-existent-host-12345.invalid",
				Port:     5432,
				User:     "test",
				Password: "test",
				Database: "testdb",
			},
		},
		{
			name: "invalid port",
			conn: &config.Connection{
				Type:     "postgres",
				Host:     "localhost",
				Port:     99999,
				User:     "test",
				Password: "test",
				Database: "testdb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := EstimatePostgreSQLSize(ctx, tt.conn)

			// Should fail due to invalid connection
			assert.Error(t, err)
			assert.Equal(t, int64(0), size)
		})
	}
}

func TestEstimateRedisSize_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		conn *config.Connection
	}{
		{
			name: "non-existent host",
			conn: &config.Connection{
				Type:     "redis",
				Host:     "non-existent-host-12345.invalid",
				Port:     6379,
				Password: "",
			},
		},
		{
			name: "invalid port",
			conn: &config.Connection{
				Type:     "redis",
				Host:     "localhost",
				Port:     99999,
				Password: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := EstimateRedisSize(ctx, tt.conn)

			// Should fail due to invalid connection
			assert.Error(t, err)
			assert.Equal(t, int64(0), size)
		})
	}
}

func TestEstimateDatabaseSize_Routing(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dbType      string
		shouldRoute bool
	}{
		{
			name:        "routes to mysql",
			dbType:      "mysql",
			shouldRoute: true,
		},
		{
			name:        "routes to postgres",
			dbType:      "postgres",
			shouldRoute: true,
		},
		{
			name:        "routes to redis",
			dbType:      "redis",
			shouldRoute: true,
		},
		{
			name:        "rejects unknown type",
			dbType:      "unknown",
			shouldRoute: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &config.Connection{
				Type:     tt.dbType,
				Host:     "non-existent-host.invalid", // Will fail but proves routing works
				Port:     1234,
				User:     "test",
				Password: "test",
				Database: "testdb",
			}

			_, err := EstimateDatabaseSize(ctx, conn)

			if tt.shouldRoute {
				// Should get error from attempting connection, not routing error
				// The error won't contain "unsupported database type"
				if err != nil {
					assert.NotContains(t, err.Error(), "unsupported database type",
						"Should route to specific function, not return unsupported error")
				}
			} else {
				// Should get unsupported type error
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported database type")
			}
		})
	}
}

func TestEstimateMySQLSize_ConnectionParameters(t *testing.T) {
	ctx := context.Background()

	conn := &config.Connection{
		Type:     "mysql",
		Host:     "testhost",
		Port:     3307,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
	}

	// This will fail (no actual MySQL), but we're testing that the function
	// accepts the connection parameters without panicking
	_, err := EstimateMySQLSize(ctx, conn)

	// Error is expected (no real MySQL server), but it shouldn't panic
	assert.Error(t, err)
}

func TestEstimatePostgreSQLSize_ConnectionParameters(t *testing.T) {
	ctx := context.Background()

	conn := &config.Connection{
		Type:     "postgres",
		Host:     "testhost",
		Port:     5433,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
	}

	// This will fail (no actual PostgreSQL), but we're testing parameter handling
	_, err := EstimatePostgreSQLSize(ctx, conn)

	// Error is expected (no real PostgreSQL server)
	assert.Error(t, err)
}

func TestEstimateRedisSize_ConnectionParameters(t *testing.T) {
	ctx := context.Background()

	conn := &config.Connection{
		Type:     "redis",
		Host:     "testhost",
		Port:     6380,
		Password: "testpass",
	}

	// This will fail (no actual Redis), but we're testing parameter handling
	_, err := EstimateRedisSize(ctx, conn)

	// Error is expected (no real Redis server)
	assert.Error(t, err)
}

func TestEstimateMySQLSize_WithoutPassword(t *testing.T) {
	ctx := context.Background()

	conn := &config.Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "", // No password
		Database: "testdb",
	}

	// Should handle missing password gracefully
	_, err := EstimateMySQLSize(ctx, conn)

	// Error expected (no MySQL server), but shouldn't panic on empty password
	assert.Error(t, err)
}

func TestEstimatePostgreSQLSize_WithoutPassword(t *testing.T) {
	ctx := context.Background()

	conn := &config.Connection{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "", // No password
		Database: "testdb",
	}

	// Should handle missing password gracefully
	_, err := EstimatePostgreSQLSize(ctx, conn)

	// Error expected (no PostgreSQL server), but shouldn't panic on empty password
	assert.Error(t, err)
}

func TestEstimateRedisSize_WithoutPassword(t *testing.T) {
	ctx := context.Background()

	conn := &config.Connection{
		Type:     "redis",
		Host:     "localhost",
		Port:     6379,
		Password: "", // No password
	}

	// Should handle missing password gracefully
	_, err := EstimateRedisSize(ctx, conn)

	// Error expected (no Redis server), but shouldn't panic on empty password
	assert.Error(t, err)
}

// NOTE: The following would require actual database instances for full integration testing:
// - TestEstimateMySQLSize_Success (requires real MySQL)
// - TestEstimatePostgreSQLSize_Success (requires real PostgreSQL)
// - TestEstimateRedisSize_Success (requires real Redis)
// - TestEstimateMySQLSize_EmptyDatabase (requires real MySQL with empty DB)
// - TestEstimatePostgreSQLSize_LargeDatabase (requires real PostgreSQL with data)
// - TestEstimateRedisSize_MemoryInfo (requires real Redis)
//
// These tests should be implemented as integration tests in a separate test suite
// with actual database containers (e.g., using testcontainers-go).
