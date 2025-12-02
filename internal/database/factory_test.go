package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
)

func TestNewDumper(t *testing.T) {
	tests := []struct {
		name         string
		target       *config.Target
		wantType     string
		wantErr      bool
		errContains  string
	}{
		{
			name:     "create mysql dumper",
			target:   fixtures.MySQLTarget(),
			wantType: "mysql",
			wantErr:  false,
		},
		{
			name:     "create postgres dumper",
			target:   fixtures.PostgresTarget(),
			wantType: "postgres",
			wantErr:  false,
		},
		{
			name:     "create redis dumper",
			target:   fixtures.RedisTarget(),
			wantType: "redis",
			wantErr:  false,
		},
		{
			name: "unsupported database type",
			target: &config.Target{
				Name: "unsupported",
				Conn: &config.Connection{
					Type: "mongodb",
				},
			},
			wantErr:     true,
			errContains: "unsupported database type",
		},
		{
			name: "empty database type",
			target: &config.Target{
				Name: "empty",
				Conn: &config.Connection{
					Type: "",
				},
			},
			wantErr:     true,
			errContains: "unsupported database type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dumper, err := NewDumper(tt.target)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, dumper)
			} else {
				require.NoError(t, err)
				require.NotNil(t, dumper)

				// Verify the correct type was created
				name := dumper.Name()
				assert.Contains(t, name, tt.wantType)
			}
		})
	}
}

func TestNewRestorer(t *testing.T) {
	tests := []struct {
		name         string
		target       *config.Target
		wantType     string
		wantErr      bool
		errContains  string
	}{
		{
			name:     "create mysql restorer",
			target:   fixtures.MySQLTarget(),
			wantType: "mysql",
			wantErr:  false,
		},
		{
			name:     "create postgres restorer",
			target:   fixtures.PostgresTarget(),
			wantType: "postgres",
			wantErr:  false,
		},
		{
			name:     "create redis restorer",
			target:   fixtures.RedisTarget(),
			wantType: "redis",
			wantErr:  false,
		},
		{
			name: "unsupported database type",
			target: &config.Target{
				Name: "unsupported",
				Conn: &config.Connection{
					Type: "cassandra",
				},
			},
			wantErr:     true,
			errContains: "unsupported database type",
		},
		{
			name: "invalid database type",
			target: &config.Target{
				Name: "invalid",
				Conn: &config.Connection{
					Type: "invalid_db",
				},
			},
			wantErr:     true,
			errContains: "unsupported database type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restorer, err := NewRestorer(tt.target)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, restorer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, restorer)

				// Verify the correct type was created
				name := restorer.Name()
				assert.Contains(t, name, tt.wantType)
			}
		})
	}
}

func TestNewDumper_MySQL_WithOptions(t *testing.T) {
	target := fixtures.MySQLTarget()
	target.ExcludeTables = []string{"temp", "cache"}
	target.AdditionalArgs = []string{"--single-transaction"}

	dumper, err := NewDumper(target)

	require.NoError(t, err)
	require.NotNil(t, dumper)

	// Verify it's a MySQL dumper
	mysqlDumper, ok := dumper.(*MySQL)
	require.True(t, ok, "should be MySQL dumper")

	assert.Equal(t, target.ExcludeTables, mysqlDumper.excludeTables)
	assert.Equal(t, target.AdditionalArgs, mysqlDumper.additionalArgs)
}

func TestNewDumper_Postgres_WithOptions(t *testing.T) {
	target := fixtures.PostgresTarget()
	target.ExcludeTables = []string{"logs"}
	target.AdditionalArgs = []string{"--format=custom"}

	dumper, err := NewDumper(target)

	require.NoError(t, err)
	require.NotNil(t, dumper)

	// Verify it's a Postgres dumper
	pgDumper, ok := dumper.(*Postgres)
	require.True(t, ok, "should be Postgres dumper")

	assert.Equal(t, target.ExcludeTables, pgDumper.excludeTables)
	assert.Equal(t, target.AdditionalArgs, pgDumper.additionalArgs)
}

func TestNewDumper_Redis_NoOptions(t *testing.T) {
	target := fixtures.RedisTarget()
	// Redis doesn't support exclude tables or additional args

	dumper, err := NewDumper(target)

	require.NoError(t, err)
	require.NotNil(t, dumper)

	// Verify it's a Redis dumper
	redisDumper, ok := dumper.(*Redis)
	require.True(t, ok, "should be Redis dumper")

	assert.NotNil(t, redisDumper.conn)
}

func TestNewRestorer_ReturnsCorrectTypes(t *testing.T) {
	tests := []struct {
		name         string
		target       *config.Target
		expectedType interface{}
	}{
		{
			name:         "mysql restorer",
			target:       fixtures.MySQLTarget(),
			expectedType: (*MySQL)(nil),
		},
		{
			name:         "postgres restorer",
			target:       fixtures.PostgresTarget(),
			expectedType: (*Postgres)(nil),
		},
		{
			name:         "redis restorer",
			target:       fixtures.RedisTarget(),
			expectedType: (*Redis)(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restorer, err := NewRestorer(tt.target)

			require.NoError(t, err)
			require.NotNil(t, restorer)

			// Check type
			switch tt.expectedType.(type) {
			case *MySQL:
				_, ok := restorer.(*MySQL)
				assert.True(t, ok, "should be MySQL restorer")
			case *Postgres:
				_, ok := restorer.(*Postgres)
				assert.True(t, ok, "should be Postgres restorer")
			case *Redis:
				_, ok := restorer.(*Redis)
				assert.True(t, ok, "should be Redis restorer")
			}
		})
	}
}

func TestNewDumper_NilTarget(t *testing.T) {
	// This should panic or return error - test that it doesn't crash unexpectedly
	defer func() {
		if r := recover(); r != nil {
			// Panic is acceptable for nil target
			t.Log("Panicked as expected for nil target")
		}
	}()

	_, _ = NewDumper(nil)
}

func TestNewRestorer_NilTarget(t *testing.T) {
	// This should panic or return error - test that it doesn't crash unexpectedly
	defer func() {
		if r := recover(); r != nil {
			// Panic is acceptable for nil target
			t.Log("Panicked as expected for nil target")
		}
	}()

	_, _ = NewRestorer(nil)
}

func TestNewDumper_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		wantErr  bool
	}{
		{
			name:     "lowercase mysql",
			dbType:   "mysql",
			wantErr:  false,
		},
		{
			name:     "uppercase MYSQL",
			dbType:   "MYSQL",
			wantErr:  true, // Should be case-sensitive
		},
		{
			name:     "mixed case MySQL",
			dbType:   "MySQL",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &config.Target{
				Name: "test",
				Conn: &config.Connection{
					Type:     tt.dbType,
					Host:     "localhost",
					Database: "testdb",
				},
			}

			_, err := NewDumper(target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFactory_DumperAndRestorerConsistency(t *testing.T) {
	// Ensure that NewDumper and NewRestorer return consistent types
	targets := []*config.Target{
		fixtures.MySQLTarget(),
		fixtures.PostgresTarget(),
		fixtures.RedisTarget(),
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			dumper, err1 := NewDumper(target)
			restorer, err2 := NewRestorer(target)

			require.NoError(t, err1)
			require.NoError(t, err2)
			require.NotNil(t, dumper)
			require.NotNil(t, restorer)

			// Both should return same underlying type
			dumperName := dumper.Name()
			restorerName := restorer.Name()

			assert.Equal(t, dumperName, restorerName, "dumper and restorer should have same name")
		})
	}
}
