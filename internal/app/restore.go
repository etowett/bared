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
func RestoreTarget(ctx context.Context, cfg *config.Config, target *config.Target, backupPath string, progress Progress) (*RestoreResult, error) {
	startTime := time.Now()
	result := &RestoreResult{
		Target:     target.Name,
		BackupPath: backupPath,
	}

	if progress != nil {
		progress.SetStage("validating", 0)
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

	log.Printf("Storage config: %+v", storageCfg)

	stor, err := storage.New(storageCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create storage: %w", err)
		return result, result.Error
	}

	log.Printf("Restoring target: %s", target.Name)
	log.Printf("Using storage backend: %s", stor.Name())
	log.Printf("Storage path: %s", storageCfg.Path)
	log.Printf("Storage bucket: %s", storageCfg.Bucket)
	log.Printf("Storage host: %s", storageCfg.Host)
	log.Printf("Storage port: %d", storageCfg.Port)
	log.Printf("Storage username: %s", storageCfg.Username)
	log.Printf("Storage password: %s", storageCfg.Password)

	result.StorageName = stor.Name()

	// Validate storage
	if err := stor.Validate(ctx); err != nil {
		result.Error = fmt.Errorf("storage validation failed: %w", err)
		return result, result.Error
	}

	// Check if backup needs decompression
	needsDecompression := strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz")
	if needsDecompression {
		log.Printf("[%s] Backup requires decompression", target.Name)
	}

	// Execute restore pipeline
	if needsDecompression {
		err = restoreWithDecompression(ctx, target, restorer, stor, backupPath)
	} else {
		err = restoreWithoutDecompression(ctx, target, restorer, stor, backupPath)
	}

	if err != nil {
		result.Error = fmt.Errorf("restore failed: %w", err)
		log.Printf("[%s] Restore failed: %v", target.Name, err)
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

	log.Printf("[%s] Starting restore pipeline: retrieve -> decompress -> restore", target.Name)

	// Start retrieval in goroutine
	go func() {
		defer retrieveWriter.Close()
		log.Printf("[%s] Retrieving backup from storage", target.Name)
		retrieveErr = stor.Retrieve(ctx, backupPath, retrieveWriter)
		if retrieveErr != nil {
			log.Printf("[%s] Retrieve error: %v", target.Name, retrieveErr)
		} else {
			log.Printf("[%s] Backup retrieval completed", target.Name)
		}
	}()

	// Start decompression in goroutine
	go func() {
		defer decompressWriter.Close()
		log.Printf("[%s] Starting decompression", target.Name)
		decompressor, err := compress.New("tgz", target.Conn.Database)
		if err != nil {
			decompressErr = err
			return
		}
		decompressErr = decompressor.Decompress(ctx, retrieveReader, decompressWriter)
		if decompressErr != nil {
			log.Printf("[%s] Decompress error: %v", target.Name, decompressErr)
		} else {
			log.Printf("[%s] Decompression completed", target.Name)
		}
	}()

	// Restore from decompressed data (this blocks until all data is read)
	log.Printf("[%s] Starting database restore", target.Name)
	restoreErr = restorer.Restore(ctx, decompressReader)
	if restoreErr != nil {
		return fmt.Errorf("restore failed: %w", restoreErr)
	}
	log.Printf("[%s] Database restore completed", target.Name)

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
func restoreWithoutDecompression(ctx context.Context, target *config.Target, restorer database.Restorer, stor storage.Storage, backupPath string) error {
	// Create pipe for streaming
	reader, writer := io.Pipe()

	var retrieveErr, restoreErr error

	log.Printf("[%s] Starting restore pipeline: retrieve -> restore", target.Name)

	// Start retrieval in goroutine
	go func() {
		defer writer.Close()
		log.Printf("[%s] Retrieving backup from storage", target.Name)
		retrieveErr = stor.Retrieve(ctx, backupPath, writer)
		if retrieveErr == nil {
			log.Printf("[%s] Backup retrieval completed", target.Name)
		}
	}()

	// Restore data (this blocks until all data is read)
	log.Printf("[%s] Starting database restore", target.Name)
	restoreErr = restorer.Restore(ctx, reader)
	if restoreErr == nil {
		log.Printf("[%s] Database restore completed", target.Name)
	}

	// Check for errors
	if retrieveErr != nil {
		return fmt.Errorf("retrieve failed: %w", retrieveErr)
	}
	if restoreErr != nil {
		return fmt.Errorf("restore failed: %w", restoreErr)
	}

	return nil
}
