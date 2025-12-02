package storage

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	bconfig "bared/internal/config"
	"bared/internal/util"
)

// S3 implements Storage for AWS S3 and S3-compatible services
type S3 struct {
	cfg    *bconfig.Storage
	client *s3.Client
}

// NewS3 creates a new S3 storage backend
func NewS3(cfg *bconfig.Storage) *S3 {
	return &S3{cfg: cfg}
}

// Name returns the storage name
func (s *S3) Name() string {
	return s.cfg.Name
}

// Validate checks if S3 is accessible
func (s *S3) Validate(ctx context.Context) error {
	if err := s.initClient(ctx); err != nil {
		return err
	}

	// Try to list objects to verify access
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access S3 bucket: %w", err)
	}

	return nil
}

// Store writes data from reader to S3
func (s *S3) Store(ctx context.Context, filePath string, r io.Reader, size int64) error {
	if err := s.initClient(ctx); err != nil {
		return err
	}

	// Combine path prefix with file path
	key := path.Join(s.cfg.Path, filePath)

	// Upload with retry
	err := util.Retry(ctx, util.DefaultRetryConfig(), func() error {
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.cfg.Bucket),
			Key:    aws.String(key),
			Body:   r,
		})
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// Retrieve reads data from S3 into writer
func (s *S3) Retrieve(ctx context.Context, filePath string, w io.Writer) error {
	if err := s.initClient(ctx); err != nil {
		return err
	}

	key := path.Join(s.cfg.Path, filePath)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	_, err = io.Copy(w, result.Body)
	if err != nil {
		return fmt.Errorf("failed to read S3 data: %w", err)
	}

	return nil
}

// List returns all backup files in S3
func (s *S3) List(ctx context.Context) ([]*BackupInfo, error) {
	if err := s.initClient(ctx); err != nil {
		return nil, err
	}

	var backups []*BackupInfo
	prefix := s.cfg.Path

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.Bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			backups = append(backups, &BackupInfo{
				Path:         *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				StorageName:  s.cfg.Name,
			})
		}
	}

	return backups, nil
}

// Delete removes a backup from S3
func (s *S3) Delete(ctx context.Context, filePath string) error {
	if err := s.initClient(ctx); err != nil {
		return err
	}

	key := path.Join(s.cfg.Path, filePath)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// initClient initializes the S3 client if not already initialized
func (s *S3) initClient(ctx context.Context) error {
	if s.client != nil {
		return nil
	}

	var cfg aws.Config
	var err error

	// If access keys are provided, use them
	if s.cfg.AccessKeyID != "" && s.cfg.SecretAccessKey != "" {
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(s.cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				s.cfg.AccessKeyID,
				s.cfg.SecretAccessKey,
				"",
			)),
		)
	} else {
		// Otherwise use default credential chain (IAM role, env vars, etc.)
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(s.cfg.Region),
		)
	}

	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	options := func(o *s3.Options) {
		// Set custom endpoint for S3-compatible services
		if s.cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(s.cfg.EndpointURL)
			o.UsePathStyle = true // Required for MinIO and many S3-compatible services
		}
	}

	s.client = s3.NewFromConfig(cfg, options)

	return nil
}
