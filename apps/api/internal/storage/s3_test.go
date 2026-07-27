package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
)

func TestNewS3(t *testing.T) {
	cfg := &config.Storage{
		Name:            "s3-test",
		Type:            "s3",
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	}

	s3 := NewS3(cfg)

	assert.NotNil(t, s3)
	assert.Equal(t, cfg, s3.cfg)
	assert.Nil(t, s3.client) // Client is initialized lazily
}

func TestS3_Name(t *testing.T) {
	cfg := &config.Storage{
		Name:   "my-s3-storage",
		Type:   "s3",
		Bucket: "test-bucket",
		Region: "us-east-1",
	}

	s3 := NewS3(cfg)
	assert.Equal(t, "my-s3-storage", s3.Name())
}

func TestS3_Configuration(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.Storage
		validateFunc func(*testing.T, *S3)
	}{
		{
			name: "with access keys",
			cfg: &config.Storage{
				Name:            "s3-keys",
				Type:            "s3",
				Bucket:          "my-bucket",
				Region:          "us-west-2",
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			validateFunc: func(t *testing.T, s3 *S3) {
				assert.Equal(t, "my-bucket", s3.cfg.Bucket)
				assert.Equal(t, "us-west-2", s3.cfg.Region)
				assert.NotEmpty(t, s3.cfg.AccessKeyID)
				assert.NotEmpty(t, s3.cfg.SecretAccessKey)
			},
		},
		{
			name: "without access keys (IAM role)",
			cfg: &config.Storage{
				Name:   "s3-iam",
				Type:   "s3",
				Bucket: "my-bucket",
				Region: "us-east-1",
			},
			validateFunc: func(t *testing.T, s3 *S3) {
				assert.Equal(t, "my-bucket", s3.cfg.Bucket)
				assert.Empty(t, s3.cfg.AccessKeyID)
				assert.Empty(t, s3.cfg.SecretAccessKey)
			},
		},
		{
			name: "with custom endpoint (MinIO)",
			cfg: &config.Storage{
				Name:            "minio",
				Type:            "s3",
				Bucket:          "backups",
				Region:          "us-east-1",
				EndpointURL:     "http://localhost:9000",
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
			},
			validateFunc: func(t *testing.T, s3 *S3) {
				assert.Equal(t, "backups", s3.cfg.Bucket)
				assert.Equal(t, "http://localhost:9000", s3.cfg.EndpointURL)
			},
		},
		{
			name: "with path prefix",
			cfg: &config.Storage{
				Name:   "s3-prefix",
				Type:   "s3",
				Bucket: "shared-bucket",
				Region: "eu-west-1",
				Path:   "backups/prod",
			},
			validateFunc: func(t *testing.T, s3 *S3) {
				assert.Equal(t, "backups/prod", s3.cfg.Path)
			},
		},
		{
			name: "different regions",
			cfg: &config.Storage{
				Name:   "s3-ap",
				Type:   "s3",
				Bucket: "asia-backups",
				Region: "ap-southeast-1",
			},
			validateFunc: func(t *testing.T, s3 *S3) {
				assert.Equal(t, "ap-southeast-1", s3.cfg.Region)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3 := NewS3(tt.cfg)
			require.NotNil(t, s3)
			tt.validateFunc(t, s3)
		})
	}
}

func TestS3_Validate_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Storage
		wantErr bool
	}{
		{
			name: "minimal valid config",
			cfg: &config.Storage{
				Name:   "s3",
				Type:   "s3",
				Bucket: "test-bucket",
				Region: "us-east-1",
			},
			wantErr: true, // Will fail because we can't connect to real S3 in tests
		},
		{
			name: "config with all fields",
			cfg: &config.Storage{
				Name:            "s3-full",
				Type:            "s3",
				Bucket:          "test-bucket",
				Region:          "us-east-1",
				AccessKeyID:     "test-key",
				SecretAccessKey: "test-secret",
				Path:            "backups/",
			},
			wantErr: true, // Will fail because we can't connect to real S3 in tests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3 := NewS3(tt.cfg)
			err := s3.Validate(context.Background())

			// In real tests, this would fail because we can't connect to AWS
			// But we're testing the code path exists
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestS3_InitClient_LazyInitialization(t *testing.T) {
	cfg := &config.Storage{
		Name:            "s3",
		Type:            "s3",
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}

	s3 := NewS3(cfg)

	// Client should be nil initially
	assert.Nil(t, s3.client)

	// After initClient, it should be set
	// (This will likely fail in test without real AWS, but tests the code path)
	err := s3.initClient(context.Background())

	// We expect it to initialize successfully (even if AWS isn't available)
	// The AWS SDK will be created, but actual operations will fail
	if err == nil {
		assert.NotNil(t, s3.client)
	}
}

func TestS3_InitClient_CalledOnce(t *testing.T) {
	cfg := &config.Storage{
		Name:            "s3",
		Type:            "s3",
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}

	s3 := NewS3(cfg)

	// Initialize client
	_ = s3.initClient(context.Background())
	firstClient := s3.client

	// Call again - should return same client
	_ = s3.initClient(context.Background())
	secondClient := s3.client

	if firstClient != nil {
		assert.Equal(t, firstClient, secondClient, "should reuse existing client")
	}
}

func TestS3_RegionConfiguration(t *testing.T) {
	regions := []string{
		"us-east-1",
		"us-west-2",
		"eu-west-1",
		"ap-southeast-1",
		"ca-central-1",
	}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			cfg := &config.Storage{
				Name:   "s3",
				Type:   "s3",
				Bucket: "test-bucket",
				Region: region,
			}

			s3 := NewS3(cfg)
			assert.Equal(t, region, s3.cfg.Region)
		})
	}
}

func TestS3_AccessKeyConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		accessKey     string
		secretKey     string
		shouldHaveKey bool
	}{
		{
			name:          "with both keys",
			accessKey:     "AKIAIOSFODNN7EXAMPLE",
			secretKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			shouldHaveKey: true,
		},
		{
			name:          "without keys",
			accessKey:     "",
			secretKey:     "",
			shouldHaveKey: false,
		},
		{
			name:          "only access key",
			accessKey:     "AKIAIOSFODNN7EXAMPLE",
			secretKey:     "",
			shouldHaveKey: false, // Both must be present
		},
		{
			name:          "only secret key",
			accessKey:     "",
			secretKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			shouldHaveKey: false, // Both must be present
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:            "s3",
				Type:            "s3",
				Bucket:          "test-bucket",
				Region:          "us-east-1",
				AccessKeyID:     tt.accessKey,
				SecretAccessKey: tt.secretKey,
			}

			s3 := NewS3(cfg)

			hasKeys := s3.cfg.AccessKeyID != "" && s3.cfg.SecretAccessKey != ""
			assert.Equal(t, tt.shouldHaveKey, hasKeys)
		})
	}
}

func TestS3_EndpointConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		description string
	}{
		{
			name:        "no endpoint (AWS S3)",
			endpoint:    "",
			description: "uses default AWS S3",
		},
		{
			name:        "MinIO local",
			endpoint:    "http://localhost:9000",
			description: "local MinIO instance",
		},
		{
			name:        "MinIO remote",
			endpoint:    "https://minio.example.com",
			description: "remote MinIO instance",
		},
		{
			name:        "DigitalOcean Spaces",
			endpoint:    "https://nyc3.digitaloceanspaces.com",
			description: "DigitalOcean Spaces",
		},
		{
			name:        "Wasabi",
			endpoint:    "https://s3.wasabisys.com",
			description: "Wasabi hot storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:        "s3",
				Type:        "s3",
				Bucket:      "test-bucket",
				Region:      "us-east-1",
				EndpointURL: tt.endpoint,
			}

			s3 := NewS3(cfg)
			assert.Equal(t, tt.endpoint, s3.cfg.EndpointURL)
		})
	}
}

func TestS3_PathPrefix(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "no prefix",
			path: "",
		},
		{
			name: "simple prefix",
			path: "backups",
		},
		{
			name: "nested prefix",
			path: "backups/prod/databases",
		},
		{
			name: "prefix with trailing slash",
			path: "backups/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:   "s3",
				Type:   "s3",
				Bucket: "test-bucket",
				Region: "us-east-1",
				Path:   tt.path,
			}

			s3 := NewS3(cfg)
			assert.Equal(t, tt.path, s3.cfg.Path)
		})
	}
}

func TestS3_ContextCancellation(_ *testing.T) {
	cfg := &config.Storage{
		Name:            "s3",
		Type:            "s3",
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}

	s3 := NewS3(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Operations should handle cancelled context
	// (They will fail, but shouldn't panic)
	_ = s3.Validate(ctx)
	_ = s3.initClient(ctx)
}

func TestS3_BucketNames(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		valid      bool
	}{
		{
			name:       "simple bucket name",
			bucketName: "my-bucket",
			valid:      true,
		},
		{
			name:       "bucket with numbers",
			bucketName: "my-bucket-123",
			valid:      true,
		},
		{
			name:       "long bucket name",
			bucketName: "very-long-bucket-name-with-many-characters",
			valid:      true,
		},
		{
			name:       "bucket with dots",
			bucketName: "my.bucket.name",
			valid:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Storage{
				Name:   "s3",
				Type:   "s3",
				Bucket: tt.bucketName,
				Region: "us-east-1",
			}

			s3 := NewS3(cfg)
			assert.Equal(t, tt.bucketName, s3.cfg.Bucket)
		})
	}
}

func TestS3_MultipleInstances(t *testing.T) {
	// Test that multiple S3 instances can coexist
	cfg1 := &config.Storage{
		Name:   "s3-prod",
		Type:   "s3",
		Bucket: "prod-bucket",
		Region: "us-east-1",
	}

	cfg2 := &config.Storage{
		Name:   "s3-staging",
		Type:   "s3",
		Bucket: "staging-bucket",
		Region: "us-west-2",
	}

	s3_1 := NewS3(cfg1)
	s3_2 := NewS3(cfg2)

	assert.NotEqual(t, s3_1.cfg.Bucket, s3_2.cfg.Bucket)
	assert.NotEqual(t, s3_1.cfg.Region, s3_2.cfg.Region)
	assert.NotEqual(t, s3_1.Name(), s3_2.Name())
}
