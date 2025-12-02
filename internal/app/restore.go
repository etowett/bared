package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"bared/internal/compress"
	"bared/internal/config"
	"bared/internal/database"
	"bared/internal/storage"
)

// RestoreResult contains information about a restore operation
type RestoreResult struct {
	Target      string
	Success     bool
	Error       error
	Duration    time.Duration
	BackupPath  string
	StorageName string
}

// RestoreTarget performs a restore for a single target
func RestoreTarget(ctx context.Context, cfg *config.Config, target *config.Target, backupPath string) (*RestoreResult, error) {
	startTime := time.Now()
	result := &RestoreResult{
		Target:     target.Name,
		BackupPath: backupPath,
	}

	log.Printf("[%s] Starting restore from: %s", target.Name, backupPath)

	// Create database restorer
	restorer, err := database.NewRestorer(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to create restorer: %w", err)
		return result, result.Error
	}

	// Get storage backend
	storageCfg, err := cfg.GetStorageForTarget(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to get storage: %w", err)
		return result, result.Error
	}

	stor, err := storage.New(storageCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create storage: %w", err)
		return result, result.Error
	}

	result.StorageName = stor.Name()

	// Validate storage
	if err := stor.Validate(ctx); err != nil {
		result.Error = fmt.Errorf("storage validation failed: %w", err)
		return result, result.Error
	}

	// Check if backup needs decompression
	needsDecompression := strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz")

	// Execute restore pipeline
	if needsDecompression {
		err = restoreWithDecompression(ctx, target, restorer, stor, backupPath)
	} else {
		err = restoreWithoutDecompression(ctx, restorer, stor, backupPath)
	}

	if err != nil {
		result.Error = fmt.Errorf("restore failed: %w", err)
		return result, result.Error
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	log.Printf("[%s] Restore completed successfully in %v", target.Name, result.Duration)

	return result, nil
}

// restoreWithDecompression performs restore with decompression
func restoreWithDecompression(ctx context.Context, target *config.Target, restorer database.Restorer, stor storage.Storage, backupPath string) error {
	// Create pipes for streaming
	retrieveReader, retrieveWriter := io.Pipe()
	decompressReader, decompressWriter := io.Pipe()

	var retrieveErr, decompressErr, restoreErr error

	// Start retrieval in goroutine
	go func() {
		defer retrieveWriter.Close()
		retrieveErr = stor.Retrieve(ctx, backupPath, retrieveWriter)
		if retrieveErr != nil {
			log.Printf("Retrieve error: %v", retrieveErr)
		}
	}()

	// Start decompression in goroutine
	go func() {
		defer decompressWriter.Close()
		decompressor, err := compress.New("tgz", target.Conn.Database)
		if err != nil {
			decompressErr = err
			return
		}
		decompressErr = decompressor.Decompress(ctx, retrieveReader, decompressWriter)
		if decompressErr != nil {
			log.Printf("Decompress error: %v", decompressErr)
		}
	}()

	// Restore from decompressed data (this blocks until all data is read)
	restoreErr = restorer.Restore(ctx, decompressReader)
	if restoreErr != nil {
		return fmt.Errorf("restore failed: %w", restoreErr)
	}

	// Check for errors in pipeline
	if retrieveErr != nil {
		return fmt.Errorf("retrieve failed: %w", retrieveErr)
	}
	if decompressErr != nil {
		return fmt.Errorf("decompress failed: %w", decompressErr)
	}

	return nil
}

// restoreWithoutDecompression performs restore without decompression
func restoreWithoutDecompression(ctx context.Context, restorer database.Restorer, stor storage.Storage, backupPath string) error {
	// Create pipe for streaming
	reader, writer := io.Pipe()

	var retrieveErr, restoreErr error

	// Start retrieval in goroutine
	go func() {
		defer writer.Close()
		retrieveErr = stor.Retrieve(ctx, backupPath, writer)
	}()

	// Restore data (this blocks until all data is read)
	restoreErr = restorer.Restore(ctx, reader)

	// Check for errors
	if retrieveErr != nil {
		return fmt.Errorf("retrieve failed: %w", retrieveErr)
	}
	if restoreErr != nil {
		return fmt.Errorf("restore failed: %w", restoreErr)
	}

	return nil
}
