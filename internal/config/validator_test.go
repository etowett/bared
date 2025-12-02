package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid minimal config",
			config: &Config{
				DefaultStorage: "local",
				Storages: map[string]*Storage{
					"local": {
						Type: "local",
						Path: "/tmp/backups",
					},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type:     "mysql",
							User:     "root",
							Password: "pass",
							Database: "testdb",
							Host:     "localhost",
							Port:     3306,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: &Config{
				DefaultStorage: "local",
				Storages: map[string]*Storage{
					"local": {
						Type: "local",
						Path: "/backups",
						Keep: 10,
					},
					"s3": {
						Type:   "s3",
						Bucket: "my-bucket",
						Region: "us-east-1",
						Keep:   20,
					},
				},
				Notifiers: map[string]*Notifier{
					"slack": {
						Type: "slack",
						URL:  "https://hooks.slack.com/services/test",
					},
				},
				Targets: []*Target{
					{
						Name: "mysql_db",
						Conn: &Connection{
							Type:     "mysql",
							User:     "root",
							Password: "pass",
							Database: "db",
							Host:     "localhost",
							Port:     3306,
						},
						Compress: &CompressionOpts{
							Enabled: true,
							Type:    "tgz",
						},
						Storage: &TargetStorage{
							Enabled: true,
							Name:    "local",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty storages",
			config: &Config{
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "at least one storage",
		},
		{
			name: "empty targets",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
			},
			wantErr:     true,
			errContains: "at least one target",
		},
		{
			name: "invalid storage type",
			config: &Config{
				DefaultStorage: "invalid",
				Storages: map[string]*Storage{
					"invalid": {
						Type: "invalid_type",
						Path: "/tmp",
					},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type:     "mysql",
							User:     "root",
							Database: "db",
							Host:     "localhost",
							Port:     3306,
						},
					},
				},
			},
			wantErr:     true,
			errContains: "unsupported type",
		},
		{
			name: "missing storage type",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {
						Path: "/tmp",
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "type is required",
		},
		{
			name: "local storage without path",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {
						Type: "local",
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name: "s3 storage without bucket",
			config: &Config{
				Storages: map[string]*Storage{
					"s3": {
						Type:   "s3",
						Region: "us-east-1",
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "bucket is required",
		},
		{
			name: "sftp storage without host",
			config: &Config{
				Storages: map[string]*Storage{
					"sftp": {
						Type:     "sftp",
						Username: "user",
						Path:     "/backups",
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "host is required",
		},
		{
			name: "negative keep value",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {
						Type: "local",
						Path: "/tmp",
						Keep: -5,
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "keep must be",
		},
		{
			name: "duplicate target names",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{Name: "duplicate", Conn: &Connection{Type: "mysql", Database: "db1"}},
					{Name: "duplicate", Conn: &Connection{Type: "mysql", Database: "db2"}},
				},
			},
			wantErr:     true,
			errContains: "duplicate target",
		},
		{
			name: "target without name",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{Name: "", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "target without connection",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{Name: "test", Conn: nil},
				},
			},
			wantErr:     true,
			errContains: "connection is required",
		},
		{
			name: "invalid database type",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type:     "invalid_db",
							Database: "db",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "connection type",
		},
		{
			name: "mysql without database",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type: "mysql",
							Host: "localhost",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "database is required",
		},
		{
			name: "postgres without database",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type: "postgres",
							Host: "localhost",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "database is required",
		},
		{
			name: "redis without host",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type: "redis",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "host is required",
		},
		{
			name: "invalid port number - negative",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type:     "mysql",
							Database: "db",
							Host:     "localhost",
							Port:     -1,
						},
					},
				},
			},
			wantErr:     true,
			errContains: "port",
		},
		{
			name: "invalid port number - too large",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{
							Type:     "mysql",
							Database: "db",
							Host:     "localhost",
							Port:     99999,
						},
					},
				},
			},
			wantErr:     true,
			errContains: "port",
		},
		{
			name: "invalid notifier type",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Notifiers: map[string]*Notifier{
					"invalid": {
						Type: "invalid_notifier",
						URL:  "http://example.com",
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "notifier type",
		},
		{
			name: "slack notifier without URL",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Notifiers: map[string]*Notifier{
					"slack": {
						Type: "slack",
					},
				},
				Targets: []*Target{
					{Name: "test", Conn: &Connection{Type: "mysql", Database: "db"}},
				},
			},
			wantErr:     true,
			errContains: "url is required",
		},
		{
			name: "target references non-existent storage",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{Type: "mysql", Database: "db"},
						Storage: &TargetStorage{
							Enabled: true,
							Name:    "nonexistent",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "storage 'nonexistent' not found",
		},
		{
			name: "invalid compression type",
			config: &Config{
				Storages: map[string]*Storage{
					"local": {Type: "local", Path: "/tmp"},
				},
				Targets: []*Target{
					{
						Name: "test",
						Conn: &Connection{Type: "mysql", Database: "db"},
						Compress: &CompressionOpts{
							Enabled: true,
							Type:    "invalid",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "compression type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err, "expected validation to fail")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains,
						"error should contain expected message")
				}
			} else {
				assert.NoError(t, err, "expected validation to pass")
			}
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	// Config with multiple validation errors
	config := &Config{
		Storages: map[string]*Storage{
			"invalid": {
				Type: "invalid_type", // Invalid type
				// Missing path for local
			},
		},
		Targets: []*Target{
			{
				Name: "test",
				Conn: &Connection{
					Type: "invalid_db", // Invalid database type
					// Missing database
				},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err, "should have validation errors")
	// The first error encountered will be returned
}
