package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"bared/internal/compress"
	"bared/internal/config"
	"bared/internal/database"
	"bared/internal/notify"
	"bared/internal/retention"
	"bared/internal/storage"
	"bared/internal/util"
)

// BackupResult contains information about a backup operation
type BackupResult struct {
	Target      string
	Success     bool
	Error       error
	Duration    time.Duration
	BackupPath  string
	Size        int64
	StorageName string
}

// countingWriter wraps an io.Writer and tracks the number of bytes written
type countingWriter struct {
	w     io.Writer
	count int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.count += int64(n)
	return n, err
}

func (cw *countingWriter) Size() int64 {
	return cw.count
}

// BackupTarget performs a backup for a single target
func BackupTarget(ctx context.Context, cfg *config.Config, target *config.Target) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{
		Target: target.Name,
	}

	log.Printf("[%s] Starting backup", target.Name)

	// Create database dumper
	dumper, err := database.NewDumper(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to create dumper: %w", err)
		return result, result.Error
	}

	// Validate dumper
	if err := dumper.Validate(ctx); err != nil {
		result.Error = fmt.Errorf("dumper validation failed: %w", err)
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

	// Generate backup path
	extension := ""
	if target.Compress != nil && target.Compress.Enabled {
		compressor, err := compress.New(target.Compress.Type, target.Conn.Database)
		if err != nil {
			result.Error = fmt.Errorf("failed to create compressor: %w", err)
			return result, result.Error
		}
		extension = compressor.Extension()
	} else {
		extension = ".sql"
	}

	backupPath := util.GenerateBackupPath(target.Name, target.Conn.Type, target.Conn.Database, extension)
	result.BackupPath = backupPath

	log.Printf("[%s] Backup details:", target.Name)
	log.Printf("\tStorage: %s", stor.Name())
	if storageCfg.Path != "" {
		log.Printf("\t\tStorage path: %s/%s", storageCfg.Path, backupPath)
	} else if storageCfg.Bucket != "" {
		log.Printf("\t\tStorage path: s3://%s/%s", storageCfg.Bucket, backupPath)
	} else if storageCfg.Host != "" {
		log.Printf("\t\tStorage path: sftp://%s@%s:%d%s/%s",
			storageCfg.Username, storageCfg.Host, storageCfg.Port, storageCfg.Path, backupPath)
	}
	log.Printf("[%s] Database: %s (type: %s, host: %s:%d, user: %s)",
		target.Name,
		target.Conn.Database,
		target.Conn.Type,
		target.Conn.Host,
		target.Conn.Port,
		target.Conn.User)
	log.Printf("[%s] Backup file: %s", target.Name, backupPath)

	// Execute backup pipeline
	var dumpMetadata *database.DumpMetadata
	if target.Compress != nil && target.Compress.Enabled {
		dumpMetadata, err = backupWithCompression(ctx, target, dumper, stor, backupPath)
	} else {
		dumpMetadata, err = backupWithoutCompression(ctx, target, dumper, stor, backupPath)
	}

	if err != nil {
		result.Error = fmt.Errorf("backup failed: %w", err)
		return result, result.Error
	}

	// Populate result with metadata
	if dumpMetadata != nil {
		result.Size = dumpMetadata.Size
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	// Update retention tracker
	tracker, err := retention.NewTracker(stor.Name(), target.Name)
	if err != nil {
		log.Printf("Warning: failed to create tracker: %v", err)
	} else {
		if err := tracker.AddBackup(backupPath, result.Size); err != nil {
			log.Printf("Warning: failed to update tracker: %v", err)
		}

		// Cleanup old backups if keep is configured
		if storageCfg.Keep > 0 {
			log.Printf("[%s] Cleaning up old backups (keeping %d)", target.Name, storageCfg.Keep)
			if err := tracker.CleanupOldBackups(stor, storageCfg.Keep); err != nil {
				log.Printf("Warning: cleanup failed: %v", err)
			}
		}
	}

	// Send notifications
	sendNotifications(ctx, cfg, target, result, nil)

	log.Printf("[%s] Backup completed successfully in %v", target.Name, result.Duration)

	return result, nil
}

// sendNotifications sends notifications to all configured notifiers
func sendNotifications(ctx context.Context, cfg *config.Config, target *config.Target, result *BackupResult, err error) {
	notifiers := cfg.GetAllNotifiers()
	if len(notifiers) == 0 {
		return
	}

	msg := &notify.Message{
		Target:    target.Name,
		Operation: "backup",
		Duration:  result.Duration,
		Path:      result.BackupPath,
		Error:     err,
		Timestamp: time.Now(),
	}

	for _, notifierCfg := range notifiers {
		notifier, notifyErr := notify.New(notifierCfg)
		if notifyErr != nil {
			log.Printf("Warning: failed to create notifier: %v", notifyErr)
			continue
		}

		// Send success or failure notification
		if err != nil {
			if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
				log.Printf("Warning: failed to send failure notification: %v", sendErr)
			}
		} else if notifier.ShouldNotifySuccess() {
			if sendErr := notifier.NotifySuccess(ctx, msg); sendErr != nil {
				log.Printf("Warning: failed to send success notification: %v", sendErr)
			}
		}
	}
}

// backupWithCompression performs backup with compression
func backupWithCompression(ctx context.Context, target *config.Target, dumper database.Dumper, stor storage.Storage, backupPath string) (*database.DumpMetadata, error) {
	// Create temp file for compressed backup
	tmpFile, cleanup, err := util.CreateBackupTempFile(target.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer cleanup()

	util.Debug("[%s] Created temp file for compression", target.Name)

	// Create pipe for streaming from dump to compression
	dumpReader, dumpWriter := io.Pipe()

	var dumpErr, compressErr error
	var dumpMetadata *database.DumpMetadata
	var uncompressedSize int64

	// Wrap dumpWriter to count uncompressed bytes
	dumpCounter := &countingWriter{w: dumpWriter}

	// Start dump in goroutine
	go func() {
		defer dumpWriter.Close()
		util.Debug("[%s] Starting database dump", target.Name)
		meta, err := dumper.Dump(ctx, dumpCounter)
		if err != nil {
			dumpErr = err
			util.Error("[%s] Dump error: %v", target.Name, err)
		} else {
			dumpMetadata = meta
			uncompressedSize = dumpCounter.Size()
			util.Info("[%s] Database dump completed: %s", target.Name, util.FormatBytes(uncompressedSize))
		}
	}()

	// Compress to temp file (blocking operation)
	util.Debug("[%s] Starting compression (type: %s)", target.Name, target.Compress.Type)
	compressor, err := compress.New(target.Compress.Type, target.Conn.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor: %w", err)
	}

	compressErr = compressor.Compress(ctx, dumpReader, tmpFile)
	if compressErr != nil {
		util.Error("[%s] Compress error: %v", target.Name, compressErr)
		return nil, fmt.Errorf("compress failed: %w", compressErr)
	}

	// Check for dump errors
	if dumpErr != nil {
		return nil, fmt.Errorf("dump failed: %w", dumpErr)
	}

	// Get compressed file size
	compressedSize, err := util.GetFileSize(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get compressed file size: %w", err)
	}

	compressionRatio := 0.0
	if uncompressedSize > 0 {
		compressionRatio = (1.0 - float64(compressedSize)/float64(uncompressedSize)) * 100
	}
	util.Info("[%s] Compression completed: %s -> %s (%.1f%% reduction)",
		target.Name, util.FormatBytes(uncompressedSize), util.FormatBytes(compressedSize), compressionRatio)

	// Seek to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	util.Debug("[%s] Uploading compressed backup to storage", target.Name)

	// Store compressed data from temp file (seekable, known size)
	storeErr := stor.Store(ctx, backupPath, tmpFile, compressedSize)
	if storeErr != nil {
		return nil, fmt.Errorf("store failed: %w", storeErr)
	}

	util.Info("[%s] Upload completed successfully", target.Name)

	// Populate metadata with file information
	if dumpMetadata == nil {
		dumpMetadata = &database.DumpMetadata{}
	}
	dumpMetadata.Size = compressedSize
	dumpMetadata.StoragePath = backupPath

	return dumpMetadata, nil
}

// backupWithoutCompression performs backup without compression
func backupWithoutCompression(ctx context.Context, target *config.Target, dumper database.Dumper, stor storage.Storage, backupPath string) (*database.DumpMetadata, error) {
	// Create temp file for backup
	tmpFile, cleanup, err := util.CreateBackupTempFile(target.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer cleanup()

	util.Debug("[%s] Created temp file for uncompressed backup", target.Name)

	// Dump directly to temp file (no goroutine needed - linear flow)
	util.Debug("[%s] Starting database dump", target.Name)
	dumpMetadata, err := dumper.Dump(ctx, tmpFile)
	if err != nil {
		util.Error("[%s] Dump error: %v", target.Name, err)
		return nil, fmt.Errorf("dump failed: %w", err)
	}

	// Get file size
	backupSize, err := util.GetFileSize(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup file size: %w", err)
	}

	util.Info("[%s] Database dump completed: %s", target.Name, util.FormatBytes(backupSize))

	// Seek to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	util.Debug("[%s] Uploading uncompressed backup to storage", target.Name)

	// Store data from temp file (seekable, known size)
	storeErr := stor.Store(ctx, backupPath, tmpFile, backupSize)
	if storeErr != nil {
		return nil, fmt.Errorf("store failed: %w", storeErr)
	}

	util.Info("[%s] Upload completed successfully", target.Name)

	// Populate metadata with file information
	if dumpMetadata == nil {
		dumpMetadata = &database.DumpMetadata{}
	}
	dumpMetadata.Size = backupSize
	dumpMetadata.StoragePath = backupPath

	return dumpMetadata, nil
}
