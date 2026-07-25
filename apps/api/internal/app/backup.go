// Package app provides high-level application logic for backup, restore, and listing operations.
package app

import (
	"context"
	"fmt"
	"io"
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
	Target     string
	Success    bool
	Error      string
	Duration   time.Duration
	BackupPath string

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

// buildStoragePath constructs a display-friendly storage path from config and backup path
func buildStoragePath(storageCfg *config.Storage, backupPath string) string {
	switch {
	case storageCfg.Path != "":
		return fmt.Sprintf("%s/%s", storageCfg.Path, backupPath)
	case storageCfg.Bucket != "":
		return fmt.Sprintf("s3://%s/%s", storageCfg.Bucket, backupPath)
	case storageCfg.Host != "":
		return fmt.Sprintf("sftp://%s@%s:%d%s/%s",
			storageCfg.Username, storageCfg.Host, storageCfg.Port, storageCfg.Path, backupPath)
	default:
		return backupPath
	}
}

// BackupTarget performs a backup for a single target
func BackupTarget(ctx context.Context, cfg *config.Config, target *config.Target, progress Progress) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{
		Target: target.Name,
	}

	logger := util.GetLogger()

	// Create stage tracker
	stageTracker := util.NewStageTracker(target.Name)
	stageTracker.StartStage("VALIDATING")

	if progress != nil {
		progress.SetStage("validating", 0)
	}

	// Generate backup path early (before any validation) so it's always set in result
	compressionEnabled := target.Compress != nil && target.Compress.Enabled
	extension := ".sql"
	if compressionEnabled {
		compressor, compressErr := compress.New(target.Compress.Type, target.Conn.Database)
		if compressErr != nil {
			// Even if compression fails, set a default path for consistency
			backupPath := util.GenerateBackupPath(target.Name, target.Conn.Type, target.Conn.Database, extension)
			result.BackupPath = backupPath
			result.Error = fmt.Errorf("failed to create compressor: %w", compressErr).Error()
			return result, fmt.Errorf("failed to create compressor: %w", compressErr)
		}
		extension = compressor.Extension()
	}

	backupPath := util.GenerateBackupPath(target.Name, target.Conn.Type, target.Conn.Database, extension)
	result.BackupPath = backupPath

	// Create database dumper
	dumper, err := database.NewDumper(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to create dumper: %w", err).Error()
		return result, fmt.Errorf("failed to create dumper: %w", err)
	}

	// Validate dumper
	if validationErr := dumper.Validate(ctx); validationErr != nil {
		result.Error = fmt.Errorf("dumper validation failed: %w", validationErr).Error()
		return result, fmt.Errorf("dumper validation failed: %w", validationErr)
	}

	// Get storage backend
	storageCfg, err := cfg.GetStorageForTarget(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to get storage: %w", err).Error()
		return result, fmt.Errorf("failed to get storage: %w", err)
	}

	// Set storage and database details early for result
	result.StorageName = storageCfg.Name
	result.StorageType = storageCfg.Type
	result.DatabaseType = target.Conn.Type
	result.DatabaseName = target.Conn.Database

	stor, err := storage.New(storageCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create storage: %w", err).Error()
		return result, fmt.Errorf("failed to create storage: %w", err)
	}

	// Validate storage
	if storageErr := stor.Validate(ctx); storageErr != nil {
		result.Error = fmt.Errorf("storage validation failed: %w", storageErr).Error()
		return result, fmt.Errorf("storage validation failed: %w", storageErr)
	}

	// Compute storage path once (used for logging and result)
	storagePath := buildStoragePath(storageCfg, backupPath)
	result.StoragePath = storagePath

	// Single consolidated log for backup configuration
	logger.InfoS("Backup job configured",
		"component", "backup",
		"target", target.Name,
		"database_type", target.Conn.Type,
		"database_name", target.Conn.Database,
		"database_host", target.Conn.Host,
		"storage_name", storageCfg.Name,
		"storage_type", storageCfg.Type,
		"backup_file", backupPath,
		"compression", compressionEnabled)

	// End validation stage
	stageTracker.EndStage(map[string]interface{}{
		"database_type": target.Conn.Type,
		"storage_type":  storageCfg.Type,
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
		wrappedErr := fmt.Errorf("backup failed: %w", err)
		result.Error = wrappedErr.Error()
		result.Stages = stageTracker.GetAllStages()

		// Send failure notifications before returning
		sendNotifications(ctx, cfg, target, result, wrappedErr)

		return result, wrappedErr
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

	// Note: result.StoragePath was already set earlier via buildStoragePath()

	result.Success = true
	result.Duration = time.Since(startTime)

	// Update retention tracker
	tracker, err := retention.NewTracker(stor.Name(), target.Name)
	if err != nil {
		logger.WarnS("Failed to create retention tracker",
			"component", "backup",
			"target", target.Name,
			"error", err)
	} else {
		if err := tracker.AddBackup(backupPath, result.Size); err != nil {
			logger.WarnS("Failed to record backup in tracker",
				"component", "backup",
				"target", target.Name,
				"error", err)
		}

		// Cleanup old backups if keep is configured
		if storageCfg.Keep > 0 {
			logger.DebugS("Running retention cleanup",
				"component", "backup",
				"target", target.Name,
				"keep_count", storageCfg.Keep)
			if err := tracker.CleanupOldBackups(stor, storageCfg.Keep); err != nil {
				logger.WarnS("Retention cleanup failed",
					"component", "backup",
					"target", target.Name,
					"error", err)
			}
		}
	}

	// Send notifications
	sendNotifications(ctx, cfg, target, result, nil)

	// Final summary log with all key metrics
	logFields := []interface{}{
		"component", "backup",
		"target", target.Name,
		"status", "success",
		"duration", result.Duration,
		"size", util.FormatBytes(result.Size),
		"storage_path", result.StoragePath,
	}

	// Add compression info if applicable
	if result.CompressionRatio > 0 {
		logFields = append(logFields,
			"uncompressed_size", util.FormatBytes(result.UncompressedSize),
			"compression_ratio", fmt.Sprintf("%.1f%%", result.CompressionRatio),
		)
	}

	logger.InfoS("Backup completed", logFields...)

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

	logger := util.GetLogger()
	for notifierName, notifierCfg := range cfg.Notifiers {
		notifier, notifyErr := notify.New(notifierCfg)
		if notifyErr != nil {
			logger.WarnS("Failed to create notifier",
				"component", "notifier",
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"destination", notifierDestination(notifierCfg),
				"error", notifyErr)
			continue
		}

		dest := notifierDestination(notifierCfg)
		start := time.Now()

		// Send success or failure notification
		if err != nil {
			logger.InfoS("Sending failure notification",
				"component", "notifier",
				"operation", msg.Operation,
				"status", "failure",
				"target", msg.Target,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"destination", dest)
			if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
				logger.ErrorS("Failed to send notification",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "failure",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"duration", time.Since(start),
					"error", sendErr)
			} else {
				logger.InfoS("Notification sent",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "failure",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"duration", time.Since(start))
			}
		} else if notifier.ShouldNotifySuccess() {
			logger.InfoS("Sending success notification",
				"component", "notifier",
				"operation", msg.Operation,
				"status", "success",
				"target", msg.Target,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"destination", dest)
			if sendErr := notifier.NotifySuccess(ctx, msg); sendErr != nil {
				logger.ErrorS("Failed to send notification",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "success",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"duration", time.Since(start),
					"error", sendErr)
			} else {
				logger.InfoS("Notification sent",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "success",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"duration", time.Since(start))
			}
		} else {
			// Avoid silent "success but no notification" confusion when on_success is disabled.
			logger.DebugS("Skipping success notification",
				"component", "notifier",
				"operation", msg.Operation,
				"status", "success",
				"target", msg.Target,
				"notifier", notifierName,
				"type", notifierCfg.Type,
				"destination", dest,
				"reason", "on_success_disabled")
		}
	}
}

// backupWithCompression performs backup with compression
func backupWithCompression(ctx context.Context, target *config.Target, dumper database.Dumper, stor storage.Storage, backupPath string, stageTracker *util.StageTracker, progress Progress) (*database.DumpMetadata, *compressionMetrics, error) {
	logger := util.GetLogger()

	// Start DUMP_AND_COMPRESS stage - dump and compression happen concurrently via streaming
	stageTracker.StartStage("DUMP_AND_COMPRESS")
	if progress != nil {
		progress.SetStage("processing", 0)
	}

	// Create temp file for compressed backup
	tmpFile, cleanup, err := util.CreateBackupTempFile(target.Name)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer cleanup()

	logger.DebugS("Temp file created for backup",
		"component", "backup",
		"target", target.Name,
		"temp_file", tmpFile.Name())

	// Create pipe for streaming from dump to compression
	dumpReader, dumpWriter := io.Pipe()

	var dumpErr error
	var dumpMetadata *database.DumpMetadata
	var uncompressedSize int64

	// Wrap dumpWriter with progress tracking
	dumpProgress := util.NewProgressWriter(dumpWriter, func(bytes int64) {
		if progress != nil {
			progress.UpdateBytes(bytes, 0) // 0 means unknown total
		}
	})

	// Start heartbeat logger for visibility during long-running operations
	heartbeat := util.NewHeartbeatLogger(
		ctx,
		target.Name,
		"DUMP_AND_COMPRESS",
		0, // Unknown total size
		func() int64 { return dumpProgress.BytesWritten() },
	)
	heartbeat.Start()
	defer heartbeat.Stop()

	// Channel to signal dump completion
	dumpDone := make(chan struct{})

	// Start dump in goroutine - streams data through pipe to compressor
	go func() {
		defer close(dumpDone)
		defer func() {
			if closeErr := dumpWriter.Close(); closeErr != nil {
				logger.DebugS("Failed to close dump writer",
					"component", "backup",
					"error", closeErr)
			}
		}()

		logger.InfoS("Dumping database",
			"component", "backup",
			"target", target.Name,
			"database_type", target.Conn.Type,
			"database_name", target.Conn.Database)

		meta, dumpResultErr := dumper.Dump(ctx, dumpProgress)
		if dumpResultErr != nil {
			dumpErr = dumpResultErr
		} else {
			dumpMetadata = meta
			uncompressedSize = dumpProgress.BytesWritten()
		}
	}()

	// Create compressor and run compression (reads from dump pipe, writes to temp file)
	compressor, err := compress.New(target.Compress.Type, target.Conn.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create compressor: %w", err)
	}

	compressErr := compressor.Compress(ctx, dumpReader, tmpFile)

	// Close reader to unblock dump goroutine if compress failed early
	if closeErr := dumpReader.Close(); closeErr != nil {
		logger.DebugS("Failed to close dump reader",
			"component", "backup",
			"error", closeErr)
	}

	// Wait for dump goroutine to complete
	<-dumpDone

	// Check for errors (compression error takes precedence as it's in main flow)
	if compressErr != nil {
		stageTracker.FailStage(compressErr)
		return nil, nil, fmt.Errorf("compress failed: %w", compressErr)
	}

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

	// End DUMP_AND_COMPRESS stage with metrics
	stageTracker.EndStage(map[string]interface{}{
		"uncompressed_size": uncompressedSize,
		"compressed_size":   compressedSize,
		"compression_ratio": compressionRatio,
	})

	// Start UPLOAD stage
	stageTracker.StartStage("UPLOAD")
	if progress != nil {
		progress.SetStage("uploading", compressedSize)
	}

	// Seek to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		stageTracker.FailStage(err)
		return nil, nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	// Store compressed data from temp file (seekable, known size)
	storeErr := stor.Store(ctx, backupPath, tmpFile, compressedSize)
	if storeErr != nil {
		stageTracker.FailStage(storeErr)
		return nil, nil, fmt.Errorf("store failed: %w", storeErr)
	}

	// End UPLOAD stage
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
	logger := util.GetLogger()

	// Start DUMP stage
	stageTracker.StartStage("DUMP")
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

	logger.InfoS("Dumping database",
		"component", "backup",
		"target", target.Name,
		"database_type", target.Conn.Type,
		"database_name", target.Conn.Database)

	// Dump directly to temp file (no goroutine needed - linear flow)
	dumpMetadata, err := dumper.Dump(ctx, tmpFile)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("dump failed: %w", err)
	}

	// Get file size
	backupSize, err := util.GetFileSize(tmpFile)
	if err != nil {
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("failed to get backup file size: %w", err)
	}

	// End DUMP stage
	stageTracker.EndStage(map[string]interface{}{
		"dump_size": backupSize,
	})

	// Start UPLOAD stage
	stageTracker.StartStage("UPLOAD")
	if progress != nil {
		progress.SetStage("uploading", backupSize)
	}

	// Seek to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		stageTracker.FailStage(err)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	// Store data from temp file (seekable, known size)
	storeErr := stor.Store(ctx, backupPath, tmpFile, backupSize)
	if storeErr != nil {
		stageTracker.FailStage(storeErr)
		return nil, fmt.Errorf("store failed: %w", storeErr)
	}

	// End UPLOAD stage
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
