package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
)

func TestNew_Local(t *testing.T) {
	cfg := &config.Storage{
		Name: "local-storage",
		Type: "local",
		Path: "/tmp/backups",
	}

	storage, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, storage)

	// Verify it's a Local storage
	local, ok := storage.(*Local)
	require.True(t, ok, "storage should be *Local type")
	assert.Equal(t, cfg, local.cfg)
	assert.Equal(t, "local-storage", storage.Name())
}

func TestNew_S3(t *testing.T) {
	cfg := &config.Storage{
		Name:            "s3-storage",
		Type:            "s3",
		Bucket:          "my-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	}

	storage, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, storage)

	// Verify it's an S3 storage
	s3, ok := storage.(*S3)
	require.True(t, ok, "storage should be *S3 type")
	assert.Equal(t, cfg, s3.cfg)
	assert.Equal(t, "s3-storage", storage.Name())
}

func TestNew_SFTP(t *testing.T) {
	cfg := &config.Storage{
		Name:     "sftp-storage",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "backup-user",
		Password: "secret",
		Path:     "/backups",
	}

	storage, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, storage)

	// Verify it's an SFTP storage
	sftp, ok := storage.(*SFTP)
	require.True(t, ok, "storage should be *SFTP type")
	assert.Equal(t, cfg, sftp.cfg)
	assert.Equal(t, "sftp-storage", storage.Name())
}

func TestNew_AllTypes(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.Storage
		expectedType string
		wantErr      bool
	}{
		{
			name: "local storage",
			cfg: &config.Storage{
				Name: "local",
				Type: "local",
				Path: "/tmp/backups",
			},
			expectedType: "*storage.Local",
			wantErr:      false,
		},
		{
			name: "s3 storage",
			cfg: &config.Storage{
				Name:   "s3",
				Type:   "s3",
				Bucket: "test-bucket",
				Region: "us-east-1",
			},
			expectedType: "*storage.S3",
			wantErr:      false,
		},
		{
			name: "sftp storage",
			cfg: &config.Storage{
				Name:     "sftp",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "user",
				Password: "pass",
				Path:     "/backups",
			},
			expectedType: "*storage.SFTP",
			wantErr:      false,
		},
		{
			name: "unsupported storage type",
			cfg: &config.Storage{
				Name: "invalid",
				Type: "ftp",
			},
			wantErr: true,
		},
		{
			name: "empty storage type",
			cfg: &config.Storage{
				Name: "empty",
				Type: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := New(tt.cfg)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, storage)
				assert.Contains(t, err.Error(), "unsupported storage type")
			} else {
				require.NoError(t, err)
				require.NotNil(t, storage)
				assert.Equal(t, tt.cfg.Name, storage.Name())
			}
		})
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
	}{
		{
			name:        "ftp type",
			storageType: "ftp",
		},
		{
			name:        "dropbox type",
			storageType: "dropbox",
		},
		{
			name:        "gdrive type",
			storageType: "gdrive",
		},
		{
			name:        "unknown type",
			storageType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name: "test",
				Type: tt.storageType,
			}

			storage, err := New(cfg)

			assert.Error(t, err)
			assert.Nil(t, storage)
			assert.Contains(t, err.Error(), "unsupported storage type")
			assert.Contains(t, err.Error(), tt.storageType)
		})
	}
}

func TestNew_CaseSensitivity(t *testing.T) {
	// Storage type should be case-sensitive
	tests := []struct {
		name        string
		storageType string
		wantErr     bool
	}{
		{
			name:        "lowercase local",
			storageType: "local",
			wantErr:     false,
		},
		{
			name:        "uppercase LOCAL",
			storageType: "LOCAL",
			wantErr:     true,
		},
		{
			name:        "mixed case Local",
			storageType: "Local",
			wantErr:     true,
		},
		{
			name:        "lowercase s3",
			storageType: "s3",
			wantErr:     false,
		},
		{
			name:        "uppercase S3",
			storageType: "S3",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:   "test",
				Type:   tt.storageType,
				Path:   "/tmp",
				Bucket: "test-bucket",
				Region: "us-east-1",
			}

			storage, err := New(cfg)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, storage)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, storage)
			}
		})
	}
}

func TestNew_LocalWithVariousConfigs(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Storage
	}{
		{
			name: "minimal local config",
			cfg: &config.Storage{
				Name: "local1",
				Type: "local",
				Path: "/tmp/backups",
			},
		},
		{
			name: "local with keep setting",
			cfg: &config.Storage{
				Name: "local2",
				Type: "local",
				Path: "/var/backups",
				Keep: 7,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := New(tt.cfg)

			require.NoError(t, err)
			require.NotNil(t, storage)

			local, ok := storage.(*Local)
			require.True(t, ok)
			assert.Equal(t, tt.cfg.Path, local.cfg.Path)
			assert.Equal(t, tt.cfg.Keep, local.cfg.Keep)
		})
	}
}

func TestNew_S3WithVariousConfigs(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Storage
	}{
		{
			name: "s3 with credentials",
			cfg: &config.Storage{
				Name:            "s3-creds",
				Type:            "s3",
				Bucket:          "my-bucket",
				Region:          "us-west-2",
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		},
		{
			name: "s3 without credentials (IAM role)",
			cfg: &config.Storage{
				Name:   "s3-iam",
				Type:   "s3",
				Bucket: "my-bucket",
				Region: "us-east-1",
			},
		},
		{
			name: "s3 with custom endpoint (MinIO)",
			cfg: &config.Storage{
				Name:            "minio",
				Type:            "s3",
				Bucket:          "backups",
				Region:          "us-east-1",
				EndpointURL:     "http://localhost:9000",
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
			},
		},
		{
			name: "s3 with path prefix",
			cfg: &config.Storage{
				Name:   "s3-prefix",
				Type:   "s3",
				Bucket: "shared-bucket",
				Region: "eu-west-1",
				Path:   "backups/prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := New(tt.cfg)

			require.NoError(t, err)
			require.NotNil(t, storage)

			s3, ok := storage.(*S3)
			require.True(t, ok)
			assert.Equal(t, tt.cfg.Bucket, s3.cfg.Bucket)
			assert.Equal(t, tt.cfg.Region, s3.cfg.Region)
			assert.Equal(t, tt.cfg.EndpointURL, s3.cfg.EndpointURL)
		})
	}
}

func TestNew_SFTPWithVariousConfigs(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Storage
	}{
		{
			name: "sftp with password",
			cfg: &config.Storage{
				Name:     "sftp-pass",
				Type:     "sftp",
				Host:     "sftp.example.com",
				Port:     22,
				Username: "backup-user",
				Password: "secret123",
				Path:     "/backups",
			},
		},
		{
			name: "sftp with custom port",
			cfg: &config.Storage{
				Name:     "sftp-custom-port",
				Type:     "sftp",
				Host:     "backup.example.com",
				Port:     2222,
				Username: "user",
				Password: "pass",
				Path:     "/var/backups",
			},
		},
		{
			name: "sftp with IP address",
			cfg: &config.Storage{
				Name:     "sftp-ip",
				Type:     "sftp",
				Host:     "192.168.1.100",
				Port:     22,
				Username: "backup",
				Password: "secret",
				Path:     "/backups",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := New(tt.cfg)

			require.NoError(t, err)
			require.NotNil(t, storage)

			sftp, ok := storage.(*SFTP)
			require.True(t, ok)
			assert.Equal(t, tt.cfg.Host, sftp.cfg.Host)
			assert.Equal(t, tt.cfg.Port, sftp.cfg.Port)
			assert.Equal(t, tt.cfg.Username, sftp.cfg.Username)
		})
	}
}

func TestNew_MultipleInstances(t *testing.T) {
	// Test that factory can create multiple independent instances
	cfg1 := &config.Storage{
		Name: "storage1",
		Type: "local",
		Path: "/tmp/backups1",
	}

	cfg2 := &config.Storage{
		Name: "storage2",
		Type: "local",
		Path: "/tmp/backups2",
	}

	cfg3 := &config.Storage{
		Name:   "storage3",
		Type:   "s3",
		Bucket: "bucket3",
		Region: "us-east-1",
	}

	storage1, err := New(cfg1)
	require.NoError(t, err)

	storage2, err := New(cfg2)
	require.NoError(t, err)

	storage3, err := New(cfg3)
	require.NoError(t, err)

	// All should be independent
	assert.NotEqual(t, storage1, storage2)
	assert.NotEqual(t, storage1, storage3)
	assert.NotEqual(t, storage2, storage3)

	// Names should be different
	assert.NotEqual(t, storage1.Name(), storage2.Name())
	assert.NotEqual(t, storage1.Name(), storage3.Name())
}

func TestNew_ErrorMessage(t *testing.T) {
	cfg := &config.Storage{
		Name: "test",
		Type: "unsupported-type",
	}

	storage, err := New(cfg)

	require.Error(t, err)
	assert.Nil(t, storage)
	assert.Equal(t, "unsupported storage type: unsupported-type", err.Error())
}
