package fixtures

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"bared/internal/config"
	"gopkg.in/yaml.v3"
)

// CreateTempConfig creates a temporary config file and returns its path
func CreateTempConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	return configPath
}

// CreateMockDump creates a mock database dump of the specified size
func CreateMockDump(size int) io.Reader {
	data := bytes.Repeat([]byte("mock dump data\n"), size/15+1)
	return bytes.NewReader(data[:size])
}

// ValidConfig returns a valid test configuration
func ValidConfig() *config.Config {
	return &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": {
				Type: "local",
				Path: "/tmp/backups",
				Keep: 5,
			},
		},
		Targets: []*config.Target{
			{
				Name: "test_db",
				Conn: &config.Connection{
					Type:     "mysql",
					User:     "root",
					Password: "testpass",
					Database: "testdb",
					Host:     "localhost",
					Port:     3306,
				},
				Compress: &config.CompressionOpts{
					Enabled: true,
					Type:    "tgz",
				},
				Storage: &config.TargetStorage{
					Enabled: true,
					Name:    "local",
				},
			},
		},
	}
}

// MinimalConfig returns a minimal valid configuration
func MinimalConfig() *config.Config {
	return &config.Config{
		Storages: map[string]*config.Storage{
			"local": {
				Type: "local",
				Path: "/tmp/backups",
			},
		},
		Targets: []*config.Target{
			{
				Name: "test",
				Conn: &config.Connection{
					Type:     "mysql",
					Database: "db",
					Host:     "localhost",
				},
			},
		},
	}
}
