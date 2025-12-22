package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	bconfig "bared/internal/config"
	"bared/internal/util"
)

const (
	// multipartThreshold is the file size above which multipart upload is used (5GB)
	// S3 has a 5GB limit for single PutObject operations
	multipartThreshold = 5 * 1024 * 1024 * 1024 // 5GB

	// multipartChunkSize is the size of each part in multipart uploads (100MB)
	// Must be between 5MB and 5GB. 100MB is a good balance for performance
	multipartChunkSize = 100 * 1024 * 1024 // 100MB

	// minMultipartSize is the minimum part size (5MB) required by S3
	minMultipartSize = 5 * 1024 * 1024 // 5MB
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

	logger := util.GetLogger()
	logger.InfoS("Uploading to S3",
		"storage", s.cfg.Name,
		"bucket", s.cfg.Bucket,
		"key", key,
		"size", util.FormatBytes(size))

	// Use multipart upload for large files (>5GB) or if size is unknown
	if size > multipartThreshold || size <= 0 {
		logger.InfoS("Using multipart upload for large file",
			"storage", s.cfg.Name,
			"component", "s3",
			"threshold", util.FormatBytes(multipartThreshold))
		return s.multipartUpload(ctx, key, r, size)
	}

	// Use regular PutObject for smaller files
	return s.singlePartUpload(ctx, key, r, size)
}

// singlePartUpload handles uploads <= 5GB using PutObject
func (s *S3) singlePartUpload(ctx context.Context, key string, r io.Reader, size int64) error {
	logger := util.GetLogger()

	// Check if reader is seekable for retry support
	seeker, isSeekable := r.(io.ReadSeeker)
	if !isSeekable {
		logger.WarnS("Reader is not seekable - retries may fail",
			"storage", s.cfg.Name,
			"component", "s3")
	} else {
		logger.DebugS("Reader is seekable - retry support enabled",
			"storage", s.cfg.Name,
			"component", "s3")
	}

	// Upload with retry
	attempt := 0
	err := util.Retry(ctx, util.DefaultRetryConfig(), func() error {
		attempt++

		// Seek to beginning for retry attempts
		if isSeekable && attempt > 1 {
			logger.DebugS("Seeking to start for retry",
				"storage", s.cfg.Name,
				"component", "s3",
				"attempt", attempt)
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
			logger.DebugS("Setting ContentLength",
				"storage", s.cfg.Name,
				"component", "s3",
				"size_bytes", size)
		}

		logger.DebugS("Starting upload attempt",
			"storage", s.cfg.Name,
			"component", "s3",
			"attempt", attempt)

		// Execute upload
		_, err := s.client.PutObject(ctx, input)
		if err != nil {
			logger.WarnS("Upload attempt failed",
				"storage", s.cfg.Name,
				"component", "s3",
				"attempt", attempt,
				"error", err)
			return fmt.Errorf("failed to upload to S3: %w", err)
		}

		logger.DebugS("Upload attempt succeeded",
			"storage", s.cfg.Name,
			"component", "s3",
			"attempt", attempt)
		return nil
	})

	if err != nil {
		logger.ErrorS("Upload failed after all retries",
			"storage", s.cfg.Name,
			"component", "s3",
			"error", err)
		return fmt.Errorf("failed to upload to S3 with retry: %w", err)
	}

	logger.InfoS("Upload completed successfully",
		"storage", s.cfg.Name,
		"component", "s3")
	return nil
}

// multipartUpload handles uploads > 5GB using multipart upload
func (s *S3) multipartUpload(ctx context.Context, key string, r io.Reader, size int64) error {
	logger := util.GetLogger()

	// Check if reader is seekable (required for multipart upload with retry)
	seeker, isSeekable := r.(io.ReadSeeker)
	if !isSeekable {
		return fmt.Errorf("multipart upload requires seekable reader")
	}

	logger.InfoS("Initiating multipart upload",
		"storage", s.cfg.Name,
		"component", "s3",
		"part_size", util.FormatBytes(multipartChunkSize))

	// Create multipart upload
	createResp, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to create multipart upload: %w", err)
	}

	uploadID := *createResp.UploadId
	logger.DebugS("Multipart upload created",
		"storage", s.cfg.Name,
		"component", "s3",
		"upload_id", uploadID)

	// Upload parts
	var completedParts []types.CompletedPart
	partNumber := int32(1)
	buf := make([]byte, multipartChunkSize)

	for {
		// Read chunk
		n, readErr := io.ReadFull(r, buf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			// Abort multipart upload on error
			logger.ErrorS("Failed to read data for multipart upload",
				"storage", s.cfg.Name,
				"component", "s3",
				"upload_id", uploadID,
				"error", readErr)
			_ = s.abortMultipartUpload(ctx, key, uploadID)
			return fmt.Errorf("failed to read data: %w", readErr)
		}

		// No more data to read
		if n == 0 {
			break
		}

		// Validate part size (must be >= 5MB except for the last part)
		if n < minMultipartSize && readErr == nil {
			logger.ErrorS("Part size too small",
				"storage", s.cfg.Name,
				"component", "s3",
				"part_number", partNumber,
				"size", n,
				"min_size", minMultipartSize)
			_ = s.abortMultipartUpload(ctx, key, uploadID)
			return fmt.Errorf("part size %d is below minimum %d", n, minMultipartSize)
		}

		logger.DebugS("Uploading part",
			"storage", s.cfg.Name,
			"component", "s3",
			"part_number", partNumber,
			"size", util.FormatBytes(int64(n)))

		// Upload part with retry
		var etag string
		err := util.Retry(ctx, util.DefaultRetryConfig(), func() error {
			// Seek back to the start of this part for retry
			if partNumber > 1 {
				offset := int64(partNumber-1) * multipartChunkSize
				if _, err := seeker.Seek(offset, 0); err != nil {
					return fmt.Errorf("failed to seek for retry: %w", err)
				}
			}

			uploadResp, uploadErr := s.client.UploadPart(ctx, &s3.UploadPartInput{
				Bucket:        aws.String(s.cfg.Bucket),
				Key:           aws.String(key),
				UploadId:      aws.String(uploadID),
				PartNumber:    aws.Int32(partNumber),
				Body:          bytes.NewReader(buf[:n]),
				ContentLength: aws.Int64(int64(n)),
			})
			if uploadErr != nil {
				logger.WarnS("Part upload failed",
					"storage", s.cfg.Name,
					"component", "s3",
					"part_number", partNumber,
					"error", uploadErr)
				return fmt.Errorf("failed to upload part: %w", uploadErr)
			}

			etag = *uploadResp.ETag
			return nil
		})

		if err != nil {
			logger.ErrorS("Part upload failed after retries",
				"storage", s.cfg.Name,
				"component", "s3",
				"part_number", partNumber,
				"upload_id", uploadID,
				"error", err)
			_ = s.abortMultipartUpload(ctx, key, uploadID)
			return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		logger.DebugS("Part uploaded successfully",
			"storage", s.cfg.Name,
			"component", "s3",
			"part_number", partNumber,
			"etag", etag)

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       aws.String(etag),
			PartNumber: aws.Int32(partNumber),
		})

		// If we hit EOF or unexpected EOF, we're done
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}

		partNumber++
	}

	logger.InfoS("All parts uploaded, completing multipart upload",
		"storage", s.cfg.Name,
		"component", "s3",
		"total_parts", len(completedParts),
		"upload_id", uploadID)

	// Complete multipart upload
	_, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.cfg.Bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		logger.ErrorS("Failed to complete multipart upload",
			"storage", s.cfg.Name,
			"component", "s3",
			"upload_id", uploadID,
			"error", err)
		_ = s.abortMultipartUpload(ctx, key, uploadID)
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	logger.InfoS("Multipart upload completed successfully",
		"storage", s.cfg.Name,
		"component", "s3",
		"total_parts", len(completedParts))
	return nil
}

// abortMultipartUpload aborts a multipart upload and cleans up resources
func (s *S3) abortMultipartUpload(ctx context.Context, key string, uploadID string) error {
	logger := util.GetLogger()
	logger.WarnS("Aborting multipart upload",
		"storage", s.cfg.Name,
		"component", "s3",
		"upload_id", uploadID)

	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.cfg.Bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		logger.ErrorS("Failed to abort multipart upload",
			"storage", s.cfg.Name,
			"component", "s3",
			"upload_id", uploadID,
			"error", err)
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

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
		//nolint:errcheck // Error closing S3 object body during cleanup is not critical
		_ = result.Body.Close()
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
		util.GetLogger().DebugS("Enabling AWS SDK debug logging",
			"storage", s.cfg.Name,
			"component", "s3")
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
