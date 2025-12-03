package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *Config)
	}{
		{
			name: "valid config",
			configData: `
default_storage: local
storages:
  local:
    type: local
    path: /tmp/backups
    keep: 5
targets:
  - name: test_db
    conn:
      type: mysql
      user: root
      password: testpass
      database: testdb
      host: localhost
      port: 3306
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "local", cfg.DefaultStorage)
				assert.Len(t, cfg.Storages, 1)
				assert.Len(t, cfg.Targets, 1)
				assert.Equal(t, "test_db", cfg.Targets[0].Name)
			},
		},
		{
			name: "config with environment variable expansion",
			configData: `
storages:
  s3:
    type: s3
    bucket: my-bucket
    access_key_id: ${TEST_AWS_KEY}
    secret_access_key: ${TEST_AWS_SECRET}
targets:
  - name: test
    conn:
      type: mysql
      database: testdb
      password: ${TEST_DB_PASSWORD}
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				// Set env vars for this test
				os.Setenv("TEST_AWS_KEY", "test_key_value")
				os.Setenv("TEST_AWS_SECRET", "test_secret_value")
				os.Setenv("TEST_DB_PASSWORD", "test_pass")
				defer func() {
					os.Unsetenv("TEST_AWS_KEY")
					os.Unsetenv("TEST_AWS_SECRET")
					os.Unsetenv("TEST_DB_PASSWORD")
				}()

				// Expand environment variables
				expandedYAML := expandEnvVars(`
storages:
  s3:
    type: s3
    bucket: my-bucket
    access_key_id: ${TEST_AWS_KEY}
    secret_access_key: ${TEST_AWS_SECRET}
targets:
  - name: test
    conn:
      type: mysql
      database: testdb
      password: ${TEST_DB_PASSWORD}
`)
				assert.Contains(t, expandedYAML, "test_key_value")
				assert.Contains(t, expandedYAML, "test_secret_value")
				assert.Contains(t, expandedYAML, "test_pass")
			},
		},
		{
			name:       "empty config",
			configData: "",
			wantErr:    false,
			validate: func(t *testing.T, cfg *Config) {
				assert.NotNil(t, cfg)
			},
		},
		{
			name: "malformed YAML",
			configData: `
storages:
  local:
    type: local
    invalid yaml here {{{}
`,
			wantErr:     true,
			errContains: "yaml",
		},
		{
			name: "duplicate target names",
			configData: `
storages:
  local:
    type: local
    path: /tmp
targets:
  - name: duplicate
    conn:
      type: mysql
      database: db1
  - name: duplicate
    conn:
      type: mysql
      database: db2
`,
			wantErr: false, // Parsing succeeds, validation would fail
			validate: func(t *testing.T, cfg *Config) {
				assert.Len(t, cfg.Targets, 2)
				// Both targets have same name (validation will catch this)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with config
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yml")
			err := os.WriteFile(configPath, []byte(tt.configData), 0644)
			require.NoError(t, err)

			// Load config
			cfg, err := Load(configPath)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/path/to/config.yml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "no such file")
}

func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "single env var",
			input:    "password: ${DB_PASSWORD}",
			envVars:  map[string]string{"DB_PASSWORD": "secret123"},
			expected: "password: secret123",
		},
		{
			name:     "multiple env vars",
			input:    "user: ${DB_USER}, password: ${DB_PASSWORD}",
			envVars:  map[string]string{"DB_USER": "admin", "DB_PASSWORD": "secret"},
			expected: "user: admin, password: secret",
		},
		{
			name:     "env var not set - remains as is",
			input:    "key: ${UNDEFINED_VAR}",
			envVars:  map[string]string{},
			expected: "key: ${UNDEFINED_VAR}",
		},
		{
			name:     "no env vars",
			input:    "plain: text",
			envVars:  map[string]string{},
			expected: "plain: text",
		},
		{
			name:     "env var with special characters",
			input:    "value: ${VAR}",
			envVars:  map[string]string{"VAR": "special!@#$%"},
			expected: "value: special!@#$%",
		},
		{
			name:     "multiple same env vars",
			input:    "${VAR} and ${VAR} again",
			envVars:  map[string]string{"VAR": "replaced"},
			expected: "replaced and replaced again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			result := expandEnvVars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandEnvVars_EmptyString(t *testing.T) {
	result := expandEnvVars("")
	assert.Equal(t, "", result)
}

func TestExpandEnvVars_NoExpansion(t *testing.T) {
	input := "no variables here"
	result := expandEnvVars(input)
	assert.Equal(t, input, result)
}

func TestExpandEnvVars_BracesOnly(t *testing.T) {
	input := "text with ${} empty braces"
	result := expandEnvVars(input)
	// Should not crash, empty var name just returns empty
	assert.Contains(t, result, "text with")
}

func TestExpandEnvVars_PartialBraces(t *testing.T) {
	input := "text with ${ unclosed"
	result := expandEnvVars(input)
	// Should not expand incomplete patterns
	assert.Equal(t, input, result)
}

func TestLoad_Integration(t *testing.T) {
	// Test loading the example config file if it exists
	examplePath := "../../../examples/config.example.yml"
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skip("Example config file not found, skipping integration test")
	}

	cfg, err := Load(examplePath)
	require.NoError(t, err, "example config should load successfully")
	require.NotNil(t, cfg)

	// Validate structure
	assert.NotEmpty(t, cfg.Storages, "example config should have storages")
	assert.NotEmpty(t, cfg.Targets, "example config should have targets")
}
