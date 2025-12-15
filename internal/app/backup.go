// Package app provides high-level application logic for backup, restore, and listing operations.
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

	// Size metrics
	Size             int64   // Compressed size (or uncompressed if no compression)
	UncompressedSize int64   // Size before compression (0 if no compression)
	CompressionRatio float64 // Percentage reduction (0 if no compression)

	// Storage details
	StorageName string
	StorageType string
	StoragePath string

	// Database details
	DatabaseType string
	DatabaseName string

	// Operation context
	Manual      bool
	ScheduledBy string

	// Stage information
	Stages []*util.Stage
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

// compressionMetrics holds compression-related metrics
type compressionMetrics struct {
	uncompressedSize int64
	compressedSize   int64
	compressionRatio float64
}

// Progress is an interface for progress tracking
type Progress interface {
	SetStage(stage string, estimatedBytes int64)
	Update(percent float64, message string)
	UpdateBytes(processed, total int64)
}

// BackupTarget performs a backup for a single target
func BackupTarget(ctx context.Context, cfg *config.Config, target *config.Target, progress Progress) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{
		Target: target.Name,
	}

	// Create stage tracker
	stageTracker := util.NewStageTracker(target.Name)
	stageTracker.StartStage("VALIDATING")

	if progress != nil {
		progress.SetStage("validating", 0)
	}

	log.Printf("[%s] Starting backup", target.Name)

	// Generate backup path early (before any validation) so it's always set in result
	extension := ""
	if target.Compress != nil && target.Compress.Enabled {
		compressor, compressErr := compress.New(target.Compress.Type, target.Conn.Database)
		if compressErr != nil {
			// Even if compression fails, set a default path for consistency
			extension = ".sql"
			backupPath := util.GenerateBackupPath(target.Name, target.Conn.Type, target.Conn.Database, extension)
			result.BackupPath = backupPath
			result.Error = fmt.Errorf("failed to create compressor: %w", compressErr)
			return result, result.Error
		}
		extension = compressor.Extension()
	} else {
		extension = ".sql"
	}

	backupPath := util.GenerateBackupPath(target.Name, target.Conn.Type, target.Conn.Database, extension)
	result.BackupPath = backupPath

	// Create database dumper
	dumper, err := database.NewDumper(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to create dumper: %w", err)
		return result, result.Error
	}

	// Validate dumper
	if validationErr := dumper.Validate(ctx); validationErr != nil {
		result.Error = fmt.Errorf("dumper validation failed: %w", validationErr)
		return result, result.Error
	}

	// Get storage backend
	storageCfg, err := cfg.GetStorageForTarget(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to get storage: %w", err)
		return result, result.Error
	}

	// Set storage details early from config
	result.StorageName = storageCfg.Name
	result.StorageType = storageCfg.Type

	// Set database details
	result.DatabaseType = target.Conn.Type
	result.DatabaseName = target.Conn.Database

	stor, err := storage.New(storageCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create storage: %w", err)
		return result, result.Error
	}

	// Validate storage
	if storageErr := stor.Validate(ctx); storageErr != nil {
		result.Error = fmt.Errorf("storage validation failed: %w", storageErr)
		return result, result.Error
	}

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

	// End validation stage
	stageTracker.EndStage(map[string]interface{}{
		"dumper_type":  target.Conn.Type,
		"storage_type": storageCfg.Type,
	})

	// Execute backup pipeline
	var dumpMetadata *database.DumpMetadata
	var compressionMetrics *compressionMetrics
	if target.Compress != nil && target.Compress.Enabled {
		dumpMetadata, compressionMetrics, err = backupWithCompression(ctx, target, dumper, stor, backupPath, stageTracker, progress)
	} else {
		dumpMetadata, err = backupWithoutCompression(ctx, target, dumper, stor, backupPath, stageTracker, progress)
	}

	if err != nil {
		stageTracker.FailStage(err)
		result.Error = fmt.Errorf("backup failed: %w", err)
		result.Stages = stageTracker.GetAllStages()
		return result, result.Error
	}

	// Populate result with metadata
	if dumpMetadata != nil {
		result.Size = dumpMetadata.Size
	}

	// Populate compression metrics if available
	if compressionMetrics != nil {
		result.UncompressedSize = compressionMetrics.uncompressedSize
		result.CompressionRatio = compressionMetrics.compressionRatio
	}

	// Store stage information
	result.Stages = stageTracker.GetAllStages()

	// Set storage path
	if storageCfg.Path != "" {
		result.StoragePath = fmt.Sprintf("%s/%s", storageCfg.Path, backupPath)
	} else if storageCfg.Bucket != "" {
		result.StoragePath = fmt.Sprintf("s3://%s/%s", storageCfg.Bucket, backupPath)
	} else if storageCfg.Host != "" {
		result.StoragePath = fmt.Sprintf("sftp://%s@%s:%d%s/%s",
			storageCfg.Username, storageCfg.Host, storageCfg.Port, storageCfg.Path, backupPath)
	} else {
		result.StoragePath = backupPath
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
	if cfg == nil || len(cfg.Notifiers) == 0 {
		return
	}

	// Build notification message with all available metrics
	msg := &notify.Message{
		// Basic information
		Target:    target.Name,
		Operation: "backup",
		Duration:  result.Duration,
		Error:     err,
		Timestamp: time.Now(),

		// File path and size metrics
		Path:             result.BackupPath,
		Size:             result.Size,
		UncompressedSize: result.UncompressedSize,
		CompressionRatio: result.CompressionRatio,

		// Storage details
		StorageName: result.StorageName,
		StorageType: result.StorageType,
		StoragePath: result.StoragePath,

		// Database details
		DatabaseType: result.DatabaseType,
		DatabaseName: result.DatabaseName,

		// Operation context
		Manual:      result.Manual,
		ScheduledBy: result.ScheduledBy,
	}

	// Convert stages to StageInfo for notifications
	if len(result.Stages) > 0 {
		msg.Stages = make([]notify.StageInfo, len(result.Stages))
		for i, stage := range result.Stages {
			msg.Stages[i] = notify.StageInfo{
				Name:     stage.Name,
				Duration: stage.Duration(),
				Status:   string(stage.Status),
			}
		}
	}

	for notifierName, notifierCfg := range cfg.Notifiers {
		notifier, notifyErr := notify.New(notifierCfg)
		if notifyErr != nil {
			log.Printf("[notify] failed to create notifier name=%s type=%s dest=%s err=%v",
				notifierName, notifierCfg.Type, notifierDestination(notifierCfg), notifyErr)
			continue
		}

		dest := notifierDestination(notifierCfg)
		start := time.Now()

		// Send success or failure notification
		if err != nil {
			log.Printf("[notify] sending op=%s status=failure target=%s notifier=%s type=%s dest=%s at=%s",
				msg.Operation, msg.Target, notifierName, notifierCfg.Type, dest, msg.Timestamp.Format(time.RFC3339))
			if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
				log.Printf("[notify] failed op=%s status=failure target=%s notifier=%s type=%s dest=%s at=%s duration=%s err=%v",
					msg.Operation, msg.Target, notifierName, notifierCfg.Type, dest, msg.Timestamp.Format(time.RFC3339), time.Since(start), sendErr)
			} else {
				log.Printf("[notify] sent op=%s status=failure target=%s notifier=%s type=%s dest=%s at=%s duration=%s",
					msg.Operation, msg.Target, notifierName, notifierCfg.Type, dest, msg.Timestamp.Format(time.RFC3339), time.Since(start))
			}
		} else if notifier.ShouldNotifySuccess() {
			log.Printf("[notify] sending op=%s status=success target=%s notifier=%s type=%s dest=%s at=%s",
				msg.Operation, msg.Target, notifierName, notifierCfg.Type, dest, msg.Timestamp.Format(time.RFC3339))
			if sendErr := notifier.NotifySuccess(ctx, msg); sendErr != nil {
				log.Printf("[notify] failed op=%s status=success target=%s notifier=%s type=%s dest=%s at=%s duration=%s err=%v",
					msg.Operation, msg.Target, notifierName, notifierCfg.Type, dest, msg.Timestamp.Format(time.RFC3339), time.Since(start), sendErr)
			} else {
				log.Printf("[notify] sent op=%s status=success target=%s notifier=%s type=%s dest=%s at=%s duration=%s",
					msg.Operation, msg.Target, notifierName, notifierCfg.Type, dest, msg.Timestamp.Format(time.RFC3339), time.Since(start))
			}
		}
	}
}

// backupWithCompression performs backup with compression
func backupWithCompression(ctx context.Context, target *config.Target, dumper database.Dumper, stor storage.Storage, backupPath string, stageTracker *util.StageTracker, progress Progress) (*database.DumpMetadata, *compressionMetrics, error) {
	// Start DUMPING stage
	stageTracker.StartStage("DUMPING")
	if progress != nil {
		progress.SetStage("dumping", 0)
	}

	// Create temp file for compressed backup
	tmpFile, cleanup, err := util.CreateBackupTempFile(target.Name)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, nil, fmt.Errorf("failed to create temp file: %w", err)
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
		defer func() {
			//nolint:govet // shadow is intentional in defer function
			if err := dumpWriter.Close(); err != nil {
				util.Warn("[%s] Failed to close dump writer: %v", target.Name, err)
			}
		}()
		util.Debug("[%s] Starting database dump", target.Name)
		meta, dumpResultErr := dumper.Dump(ctx, dumpCounter)
		if dumpResultErr != nil {
			dumpErr = dumpResultErr
			util.Error("[%s] Dump error: %v", target.Name, dumpResultErr)
		} else {
			dumpMetadata = meta
			uncompressedSize = dumpCounter.Size()
			util.Info("[%s] Database dump completed: %s", target.Name, util.FormatBytes(uncompressedSize))
		}
	}()

	// Start COMPRESSING stage
	stageTracker.StartStage("COMPRESSING")
	if progress != nil {
		progress.SetStage("compressing", 0)
	}

	util.Debug("[%s] Starting compression (type: %s)", target.Name, target.Compress.Type)
	compressor, err := compress.New(target.Compress.Type, target.Conn.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create compressor: %w", err)
	}

	compressErr = compressor.Compress(ctx, dumpReader, tmpFile)
	if compressErr != nil {
		util.Error("[%s] Compress error: %v", target.Name, compressErr)
		stageTracker.FailStage(compressErr)
		return nil, nil, fmt.Errorf("compress failed: %w", compressErr)
	}

	// Check for dump errors
	if dumpErr != nil {
		stageTracker.FailStage(dumpErr)
		return nil, nil, fmt.Errorf("dump failed: %w", dumpErr)
	}

	// Get compressed file size
	compressedSize, err := util.GetFileSize(tmpFile)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, nil, fmt.Errorf("failed to get compressed file size: %w", err)
	}

	compressionRatio := 0.0
	if uncompressedSize > 0 {
		compressionRatio = (1.0 - float64(compressedSize)/float64(uncompressedSize)) * 100
	}
	util.Info("[%s] Compression completed: %s -> %s (%.1f%% reduction)",
		target.Name, util.FormatBytes(uncompressedSize), util.FormatBytes(compressedSize), compressionRatio)

	// End COMPRESSING stage
	stageTracker.EndStage(map[string]interface{}{
		"uncompressed_size": uncompressedSize,
		"compressed_size":   compressedSize,
		"compression_ratio": compressionRatio,
	})

	// Start UPLOADING stage
	stageTracker.StartStage("UPLOADING")
	if progress != nil {
		progress.SetStage("uploading", compressedSize)
	}

	// Seek to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		stageTracker.FailStage(err)
		return nil, nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	util.Debug("[%s] Uploading compressed backup to storage", target.Name)

	// Store compressed data from temp file (seekable, known size)
	storeErr := stor.Store(ctx, backupPath, tmpFile, compressedSize)
	if storeErr != nil {
		stageTracker.FailStage(storeErr)
		return nil, nil, fmt.Errorf("store failed: %w", storeErr)
	}

	util.Info("[%s] Upload completed successfully", target.Name)

	// End UPLOADING stage
	stageTracker.EndStage(map[string]interface{}{
		"uploaded_size": compressedSize,
	})

	// Populate metadata with file information
	if dumpMetadata == nil {
		dumpMetadata = &database.DumpMetadata{}
	}
	dumpMetadata.Size = compressedSize
	dumpMetadata.StoragePath = backupPath

	// Create compression metrics
	metrics := &compressionMetrics{
		uncompressedSize: uncompressedSize,
		compressedSize:   compressedSize,
		compressionRatio: compressionRatio,
	}

	return dumpMetadata, metrics, nil
}

// backupWithoutCompression performs backup without compression
func backupWithoutCompression(ctx context.Context, target *config.Target, dumper database.Dumper, stor storage.Storage, backupPath string, stageTracker *util.StageTracker, progress Progress) (*database.DumpMetadata, error) {
	// Start DUMPING stage
	stageTracker.StartStage("DUMPING")
	if progress != nil {
		progress.SetStage("dumping", 0)
	}

	// Create temp file for backup
	tmpFile, cleanup, err := util.CreateBackupTempFile(target.Name)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer cleanup()

	util.Debug("[%s] Created temp file for uncompressed backup", target.Name)

	// Dump directly to temp file (no goroutine needed - linear flow)
	util.Debug("[%s] Starting database dump", target.Name)
	dumpMetadata, err := dumper.Dump(ctx, tmpFile)
	if err != nil {
		util.Error("[%s] Dump error: %v", target.Name, err)
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("dump failed: %w", err)
	}

	// Get file size
	backupSize, err := util.GetFileSize(tmpFile)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("failed to get backup file size: %w", err)
	}

	util.Info("[%s] Database dump completed: %s", target.Name, util.FormatBytes(backupSize))

	// End DUMPING stage
	stageTracker.EndStage(map[string]interface{}{
		"dump_size": backupSize,
	})

	// Start UPLOADING stage
	stageTracker.StartStage("UPLOADING")
	if progress != nil {
		progress.SetStage("uploading", backupSize)
	}

	// Seek to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	util.Debug("[%s] Uploading uncompressed backup to storage", target.Name)

	// Store data from temp file (seekable, known size)
	storeErr := stor.Store(ctx, backupPath, tmpFile, backupSize)
	if storeErr != nil {
		stageTracker.FailStage(storeErr)
		return nil, fmt.Errorf("store failed: %w", storeErr)
	}

	util.Info("[%s] Upload completed successfully", target.Name)

	// End UPLOADING stage
	stageTracker.EndStage(map[string]interface{}{
		"uploaded_size": backupSize,
	})

	// Populate metadata with file information
	if dumpMetadata == nil {
		dumpMetadata = &database.DumpMetadata{}
	}
	dumpMetadata.Size = backupSize
	dumpMetadata.StoragePath = backupPath

	return dumpMetadata, nil
}
