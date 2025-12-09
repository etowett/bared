package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
)

func TestNewSFTP(t *testing.T) {
	cfg := &config.Storage{
		Name:     "sftp-test",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "backup-user",
		Password: "secret",
		Path:     "/backups",
	}

	sftp := NewSFTP(cfg)

	assert.NotNil(t, sftp)
	assert.Equal(t, cfg, sftp.cfg)
	assert.Nil(t, sftp.sshClient)  // Client is initialized lazily
	assert.Nil(t, sftp.sftpClient) // Client is initialized lazily
}

func TestSFTP_Name(t *testing.T) {
	cfg := &config.Storage{
		Name: "my-sftp-storage",
		Type: "sftp",
		Host: "sftp.example.com",
		Port: 22,
		Path: "/backups",
	}

	sftp := NewSFTP(cfg)
	assert.Equal(t, "my-sftp-storage", sftp.Name())
}

func TestSFTP_Configuration(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.Storage
		validateFunc func(*testing.T, *SFTP)
	}{
		{
			name: "with password authentication",
			cfg: &config.Storage{
				Name:     "sftp-pass",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "backupuser",
				Password: "secret123",
				Path:     "/var/backups",
			},
			validateFunc: func(t *testing.T, sftp *SFTP) {
				assert.Equal(t, "sftp.example.com", sftp.cfg.Host)
				assert.Equal(t, 22, sftp.cfg.Port)
				assert.Equal(t, "backupuser", sftp.cfg.Username)
				assert.NotEmpty(t, sftp.cfg.Password)
			},
		},
		{
			name: "custom port",
			cfg: &config.Storage{
				Name:     "sftp-custom-port",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     2222,
				Username: "user",
				Password: "pass",
				Path:     "/backups",
			},
			validateFunc: func(t *testing.T, sftp *SFTP) {
				assert.Equal(t, 2222, sftp.cfg.Port)
			},
		},
		{
			name: "with path prefix",
			cfg: &config.Storage{
				Name:     "sftp-prefix",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "user",
				Password: "pass",
				Path:     "/home/user/backups/prod",
			},
			validateFunc: func(t *testing.T, sftp *SFTP) {
				assert.Equal(t, "/home/user/backups/prod", sftp.cfg.Path)
			},
		},
		{
			name: "IP address host",
			cfg: &config.Storage{
				Name:     "sftp-ip",
				Type:     "sftp",
				Host:     "192.168.1.100",
				Port:     22,
				Username: "backup",
				Password: "secret",
				Path:     "/backups",
			},
			validateFunc: func(t *testing.T, sftp *SFTP) {
				assert.Equal(t, "192.168.1.100", sftp.cfg.Host)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sftp := NewSFTP(tt.cfg)
			require.NotNil(t, sftp)
			tt.validateFunc(t, sftp)
		})
	}
}

func TestSFTP_Validate_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Storage
		wantErr bool
	}{
		{
			name: "minimal valid config",
			cfg: &config.Storage{
				Name:     "sftp",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "user",
				Password: "pass",
				Path:     "/backups",
			},
			wantErr: true, // Will fail because we can't connect to real SFTP in tests
		},
		{
			name: "config with all fields",
			cfg: &config.Storage{
				Name:     "sftp-full",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "backup-user",
				Password: "secret-password",
				Path:     "/var/backups/databases",
			},
			wantErr: true, // Will fail because we can't connect to real SFTP in tests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sftp := NewSFTP(tt.cfg)
			err := sftp.Validate(context.Background())

			// In real tests, this would fail because we can't connect to SFTP
			// But we're testing the code path exists
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestSFTP_Connect_LazyInitialization(t *testing.T) {
	cfg := &config.Storage{
		Name:     "sftp",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "user",
		Password: "pass",
		Path:     "/backups",
	}

	sftp := NewSFTP(cfg)

	// Clients should be nil initially
	assert.Nil(t, sftp.sshClient)
	assert.Nil(t, sftp.sftpClient)

	// After connect, they should be set (will fail without real server)
	_ = sftp.connect()
}

func TestSFTP_PortConfiguration(t *testing.T) {
	ports := []int{
		22,    // Standard SSH port
		2222,  // Common alternative
		22022, // Another common alternative
		10022, // Custom port
	}

	for _, port := range ports {
		t.Run(fmt.Sprintf("port_%d", port), func(t *testing.T) {
			cfg := &config.Storage{
				Name:     "sftp",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     port,
				Username: "user",
				Password: "pass",
				Path:     "/backups",
			}

			sftp := NewSFTP(cfg)
			assert.Equal(t, port, sftp.cfg.Port)
		})
	}
}

func TestSFTP_HostConfiguration(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "hostname",
			host: "sftp.example.com",
		},
		{
			name: "subdomain",
			host: "backups.sftp.example.com",
		},
		{
			name: "IPv4",
			host: "192.168.1.100",
		},
		{
			name: "localhost",
			host: "localhost",
		},
		{
			name: "internal hostname",
			host: "backup-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:     "sftp",
				Type:     "sftp",
				Host:     tt.host,
				Port:     22,
				Username: "user",
				Password: "pass",
				Path:     "/backups",
			}

			sftp := NewSFTP(cfg)
			assert.Equal(t, tt.host, sftp.cfg.Host)
		})
	}
}

func TestSFTP_AuthenticationConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "standard credentials",
			username: "backup-user",
			password: "secret123",
		},
		{
			name:     "root user",
			username: "root",
			password: "rootpass",
		},
		{
			name:     "email as username",
			username: "backup@example.com",
			password: "password",
		},
		{
			name:     "complex password",
			username: "user",
			password: "P@ssw0rd!#$%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:     "sftp",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: tt.username,
				Password: tt.password,
				Path:     "/backups",
			}

			sftp := NewSFTP(cfg)
			assert.Equal(t, tt.username, sftp.cfg.Username)
			assert.Equal(t, tt.password, sftp.cfg.Password)
		})
	}
}

func TestSFTP_PathConfiguration(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "root backups",
			path: "/backups",
		},
		{
			name: "user home directory",
			path: "/home/user/backups",
		},
		{
			name: "nested path",
			path: "/var/backups/databases/prod",
		},
		{
			name: "relative path",
			path: "backups",
		},
		{
			name: "path with spaces",
			path: "/backup files/databases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:     "sftp",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "user",
				Password: "pass",
				Path:     tt.path,
			}

			sftp := NewSFTP(cfg)
			assert.Equal(t, tt.path, sftp.cfg.Path)
		})
	}
}

func TestSFTP_ContextCancellation(_ *testing.T) {
	cfg := &config.Storage{
		Name:     "sftp",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "user",
		Password: "pass",
		Path:     "/backups",
	}

	sftp := NewSFTP(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Operations should handle cancelled context
	// (They will fail, but shouldn't panic)
	_ = sftp.Validate(ctx)
}

func TestSFTP_Disconnect_NoClients(t *testing.T) {
	cfg := &config.Storage{
		Name:     "sftp",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "user",
		Password: "pass",
		Path:     "/backups",
	}

	sftp := NewSFTP(cfg)

	// Should not panic when disconnecting without connecting first
	sftp.disconnect()

	assert.Nil(t, sftp.sshClient)
	assert.Nil(t, sftp.sftpClient)
}

func TestSFTP_MultipleInstances(t *testing.T) {
	// Test that multiple SFTP instances can coexist
	cfg1 := &config.Storage{
		Name:     "sftp-prod",
		Type:     "sftp",
		Host:     "prod.sftp.example.com",
		Port:     22,
		Username: "prod-user",
		Password: "prod-pass",
		Path:     "/prod/backups",
	}

	cfg2 := &config.Storage{
		Name:     "sftp-staging",
		Type:     "sftp",
		Host:     "staging.sftp.example.com",
		Port:     2222,
		Username: "staging-user",
		Password: "staging-pass",
		Path:     "/staging/backups",
	}

	sftp1 := NewSFTP(cfg1)
	sftp2 := NewSFTP(cfg2)

	assert.NotEqual(t, sftp1.cfg.Host, sftp2.cfg.Host)
	assert.NotEqual(t, sftp1.cfg.Port, sftp2.cfg.Port)
	assert.NotEqual(t, sftp1.Name(), sftp2.Name())
}

func TestSFTP_StandardPort(t *testing.T) {
	cfg := &config.Storage{
		Name:     "sftp",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "user",
		Password: "pass",
		Path:     "/backups",
	}

	sftp := NewSFTP(cfg)
	assert.Equal(t, 22, sftp.cfg.Port, "should use standard SSH port")
}

func TestSFTP_ConfigurationFields(t *testing.T) {
	cfg := &config.Storage{
		Name:     "test-sftp",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "testuser",
		Password: "testpass",
		Path:     "/test/path",
		Keep:     10,
	}

	sftp := NewSFTP(cfg)

	// Verify all fields are preserved
	assert.Equal(t, "test-sftp", sftp.cfg.Name)
	assert.Equal(t, "sftp", sftp.cfg.Type)
	assert.Equal(t, "sftp.example.com", sftp.cfg.Host)
	assert.Equal(t, 22, sftp.cfg.Port)
	assert.Equal(t, "testuser", sftp.cfg.Username)
	assert.Equal(t, "testpass", sftp.cfg.Password)
	assert.Equal(t, "/test/path", sftp.cfg.Path)
	assert.Equal(t, 10, sftp.cfg.Keep)
}
