package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

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
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Combine path prefix with file path
	key := path.Join(s.cfg.Path, filePath)

	// Check if reader is seekable for retry support
	seeker, isSeekable := r.(io.ReadSeeker)
	if !isSeekable {
		util.Warn("[S3:%s] Reader is not seekable - retries may fail", s.cfg.Name)
	} else {
		util.Debug("[S3:%s] Reader is seekable - retry support enabled", s.cfg.Name)
	}

	util.Info("[S3:%s] Uploading to bucket '%s' key '%s' (%s)",
		s.cfg.Name, s.cfg.Bucket, key, util.FormatBytes(size))

	// Upload with retry
	attempt := 0
	err := util.Retry(ctx, util.DefaultRetryConfig(), func() error {
		attempt++

		// Seek to beginning for retry attempts
		if isSeekable && attempt > 1 {
			util.Debug("[S3:%s] Seeking to start for retry attempt %d", s.cfg.Name, attempt)
			if _, err := seeker.Seek(0, 0); err != nil {
				return fmt.Errorf("failed to seek reader for retry: %w", err)
			}
		}

		// Prepare upload input
		input := &s3.PutObjectInput{
			Bucket: aws.String(s.cfg.Bucket),
			Key:    aws.String(key),
			Body:   r,
		}

		// Set ContentLength if size is known (CRITICAL for Hetzner and S3-compatible services)
		if size > 0 {
			input.ContentLength = aws.Int64(size)
			util.Debug("[S3:%s] Setting ContentLength: %d bytes", s.cfg.Name, size)
		}

		util.Debug("[S3:%s] Starting upload attempt %d", s.cfg.Name, attempt)

		// Execute upload
		_, err := s.client.PutObject(ctx, input)
		if err != nil {
			util.Warn("[S3:%s] Upload attempt %d failed: %v", s.cfg.Name, attempt, err)
			return fmt.Errorf("failed to upload to S3: %w", err)
		}

		util.Debug("[S3:%s] Upload attempt %d succeeded", s.cfg.Name, attempt)
		return nil
	})

	if err != nil {
		util.Error("[S3:%s] Upload failed after all retries: %v", s.cfg.Name, err)
		return fmt.Errorf("failed to upload to S3 with retry: %w", err)
	}

	util.Info("[S3:%s] Upload completed successfully", s.cfg.Name)
	return nil
}

// Retrieve reads data from S3 into writer
func (s *S3) Retrieve(ctx context.Context, filePath string, w io.Writer) error {
	if err := s.initClient(ctx); err != nil {
		return err
	}

	// Check if filePath already includes the storage path prefix
	var key string
	if s.cfg.Path != "" && strings.HasPrefix(filePath, s.cfg.Path+"/") {
		// Path already includes storage prefix, use as-is
		key = filePath
	} else {
		// Path is relative, prepend storage path
		key = path.Join(s.cfg.Path, filePath)
	}

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download from S3: %w", err)
	}
	defer func() {
		//nolint:govet,staticcheck // shadow is intentional; empty branch is intentional - error handling in defer
		if err := result.Body.Close(); err != nil {
			// Error already being returned by main function
		}
	}()

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

	// Check if filePath already includes the storage path prefix
	var key string
	if s.cfg.Path != "" && strings.HasPrefix(filePath, s.cfg.Path+"/") {
		// Path already includes storage prefix, use as-is
		key = filePath
	} else {
		// Path is relative, prepend storage path
		key = path.Join(s.cfg.Path, filePath)
	}

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

	// Build config options
	configOptions := []func(*config.LoadOptions) error{
		config.WithRegion(s.cfg.Region),
	}

	// Enable SDK debug logging if log level is DEBUG
	if util.GetLogger().GetLevel() == util.DEBUG {
		util.Debug("[S3:%s] Enabling AWS SDK debug logging", s.cfg.Name)
		configOptions = append(configOptions, config.WithClientLogMode(
			aws.LogRequestWithBody|aws.LogResponseWithBody|aws.LogRetries,
		))
	}

	// If access keys are provided, use them
	if s.cfg.AccessKeyID != "" && s.cfg.SecretAccessKey != "" {
		configOptions = append(configOptions, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s.cfg.AccessKeyID,
			s.cfg.SecretAccessKey,
			"",
		)))
		cfg, err = config.LoadDefaultConfig(ctx, configOptions...)
	} else {
		// Otherwise use default credential chain (IAM role, env vars, etc.)
		cfg, err = config.LoadDefaultConfig(ctx, configOptions...)
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

// Exists checks if a backup file exists in S3
func (s *S3) Exists(ctx context.Context, filePath string) (bool, error) {
	if err := s.initClient(ctx); err != nil {
		return false, err
	}

	// Check if filePath already includes the storage path prefix
	var key string
	if s.cfg.Path != "" && strings.HasPrefix(filePath, s.cfg.Path+"/") {
		key = filePath
	} else {
		key = path.Join(s.cfg.Path, filePath)
	}

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetInfo returns metadata about a backup file in S3
func (s *S3) GetInfo(ctx context.Context, filePath string) (*BackupInfo, error) {
	if err := s.initClient(ctx); err != nil {
		return nil, err
	}

	// Check if filePath already includes the storage path prefix
	var key string
	if s.cfg.Path != "" && strings.HasPrefix(filePath, s.cfg.Path+"/") {
		key = filePath
	} else {
		key = path.Join(s.cfg.Path, filePath)
	}

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &BackupInfo{
		Path:         filePath,
		Size:         *result.ContentLength,
		LastModified: *result.LastModified,
		StorageName:  s.cfg.Name,
	}, nil
}
