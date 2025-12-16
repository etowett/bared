package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"bared/internal/compress"
	"bared/internal/config"
	"bared/internal/database"
	"bared/internal/notify"
	"bared/internal/storage"
	"bared/internal/util"
)

// RestoreOptions contains options for restore operations
type RestoreOptions struct {
	DryRun           bool // If true, validate only without executing restore
	SkipValidation   bool // Skip connection validation (not recommended)
	SkipBackupVerify bool // Skip backup file verification
}

// RestoreResult contains information about a restore operation
type RestoreResult struct {
	Target     string
	Success    bool
	Error      string
	Duration   time.Duration
	BackupPath string

	// Storage details
	StorageName string
	StorageType string
	BackupSize  int64

	// Validation details
	DryRun            bool
	Validations       []string
	ValidationsPassed int

	// Database details
	DatabaseType  string
	DatabaseName  string
	RestoredBytes int64

	// Operation context
	Manual      bool
	ScheduledBy string

	// Stage information
	Stages []*util.Stage
}

// RestoreTargetWithOptions performs a restore with specified options
func RestoreTargetWithOptions(ctx context.Context, cfg *config.Config, target *config.Target, backupPath string, options *RestoreOptions, progress Progress) (*RestoreResult, error) {
	logger := util.GetLogger()
	startTime := time.Now()

	// Initialize options if nil
	if options == nil {
		options = &RestoreOptions{}
	}

	result := &RestoreResult{
		Target:      target.Name,
		BackupPath:  backupPath,
		DryRun:      options.DryRun,
		Validations: []string{},
	}

	// Create stage tracker
	stageTracker := util.NewStageTracker(target.Name)
	stageTracker.StartStage("VALIDATING")

	if progress != nil {
		progress.SetStage("validating", 0)
	}

	mode := "live"
	if result.DryRun {
		mode = "dry-run"
	}
	logger.InfoS("Starting restore",
		"component", "restore",
		"target", target.Name,
		"mode", mode,
		"backup_path", backupPath)

	// Step 1: Create database restorer
	restorer, err := database.NewRestorer(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to create restorer: %w", err).Error()
		return result, fmt.Errorf("failed to create restorer: %w", err)
	}
	result.Validations = append(result.Validations, "Database restorer created")

	// Step 2: Validate database connection (unless skipped)
	if !options.SkipValidation {
		logger.InfoS("Validating database connection",
			"component", "restore",
			"target", target.Name)
		if connErr := restorer.ValidateConnection(ctx); connErr != nil {
			result.Error = fmt.Errorf("database connection validation failed: %w", connErr).Error()
			return result, fmt.Errorf("database connection validation failed: %w", connErr)
		}
		result.Validations = append(result.Validations, "Database connection validated")
		logger.InfoS("Database connection validated successfully",
			"component", "restore",
			"target", target.Name)
	}

	// Step 3: Get storage backend
	storageCfg, err := cfg.GetStorageForTarget(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to get storage: %w", err).Error()
		return result, fmt.Errorf("failed to get storage: %w", err)
	}

	stor, err := storage.New(storageCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create storage: %w", err).Error()
		return result, fmt.Errorf("failed to create storage: %w", err)
	}

	// Set storage details
	result.StorageName = stor.Name()
	result.StorageType = storageCfg.Type
	result.Validations = append(result.Validations, fmt.Sprintf("Storage backend connected: %s", stor.Name()))

	// Set database details
	result.DatabaseType = target.Conn.Type
	result.DatabaseName = target.Conn.Database

	// Step 4: Validate storage
	if err := stor.Validate(ctx); err != nil {
		result.Error = fmt.Errorf("storage validation failed: %w", err).Error()
		return result, fmt.Errorf("storage validation failed: %w", err)
	}
	result.Validations = append(result.Validations, "Storage validated")

	// Step 5: Verify backup file exists (unless skipped)
	if !options.SkipBackupVerify {
		logger.InfoS("Verifying backup file exists",
			"component", "restore",
			"target", target.Name)
		exists, err := stor.Exists(ctx, backupPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to verify backup existence: %w", err).Error()
			return result, fmt.Errorf("failed to verify backup existence: %w", err)
		}
		if !exists {
			result.Error = fmt.Errorf("backup file not found: %s", backupPath).Error()
			return result, fmt.Errorf("backup file not found: %s", backupPath)
		}

		// Get backup file info
		backupInfo, err := stor.GetInfo(ctx, backupPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to get backup info: %w", err).Error()
			return result, fmt.Errorf("failed to get backup info: %w", err)
		}

		// Store backup size
		result.BackupSize = backupInfo.Size

		result.Validations = append(result.Validations,
			fmt.Sprintf("Backup file verified: %s (size: %d bytes, modified: %s)",
				backupPath, backupInfo.Size, backupInfo.LastModified.Format(time.RFC3339)))
		logger.InfoS("Backup file verified",
			"component", "restore",
			"target", target.Name,
			"size_bytes", backupInfo.Size)
	}

	// Step 6: Check if backup needs decompression
	needsDecompression := strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz")
	if needsDecompression {
		result.Validations = append(result.Validations, "Backup requires decompression")
		logger.InfoS("Backup requires decompression",
			"component", "restore",
			"target", target.Name)
	}

	// End validation stage
	stageTracker.EndStage(map[string]interface{}{
		"validations_passed":  len(result.Validations),
		"needs_decompression": needsDecompression,
	})

	// Store validations count
	result.ValidationsPassed = len(result.Validations)

	// If dry-run, stop here
	if result.DryRun {
		result.Success = true
		result.Duration = time.Since(startTime)
		result.Stages = stageTracker.GetAllStages()
		logger.InfoS("DRY-RUN completed successfully",
			"component", "restore",
			"target", target.Name,
			"validations_passed", len(result.Validations))
		logger.InfoS("Validations performed",
			"component", "restore",
			"target", target.Name)
		for _, v := range result.Validations {
			logger.InfoS("Validation",
				"component", "restore",
				"target", target.Name,
				"validation", v)
		}
		return result, nil
	}

	// Step 7: Execute actual restore pipeline
	if progress != nil {
		progress.SetStage("restoring", 10)
	}

	var restoreErr error
	if needsDecompression {
		restoreErr = restoreWithDecompression(ctx, target, restorer, stor, backupPath, stageTracker, progress)
	} else {
		restoreErr = restoreWithoutDecompression(ctx, target, restorer, stor, backupPath, stageTracker, progress)
	}

	if restoreErr != nil {
		result.Error = fmt.Errorf("restore failed: %w", restoreErr).Error()
		result.Duration = time.Since(startTime)
		result.Stages = stageTracker.GetAllStages()
		logger.ErrorS("Restore failed",
			"component", "restore",
			"target", target.Name,
			"error", restoreErr)

		// Send failure notifications
		sendRestoreNotifications(ctx, cfg, target, result, restoreErr)

		return result, fmt.Errorf("restore failed: %w", restoreErr)
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	// Store stage information
	result.Stages = stageTracker.GetAllStages()

	logger.InfoS("Restore completed successfully",
		"component", "restore",
		"target", target.Name,
		"duration", result.Duration)

	// Send success notifications
	sendRestoreNotifications(ctx, cfg, target, result, nil)

	return result, nil
}

// RestoreTarget performs a restore (backward compatible wrapper)
func RestoreTarget(ctx context.Context, cfg *config.Config, target *config.Target, backupPath string, progress Progress) (*RestoreResult, error) {
	return RestoreTargetWithOptions(ctx, cfg, target, backupPath, &RestoreOptions{}, progress)
}

// restoreWithDecompression performs restore with decompression
func restoreWithDecompression(ctx context.Context, target *config.Target, restorer database.Restorer, stor storage.Storage, backupPath string, stageTracker *util.StageTracker, progress Progress) error {
	logger := util.GetLogger()

	// Start RETRIEVING stage
	stageTracker.StartStage("RETRIEVING")
	if progress != nil {
		progress.SetStage("retrieving", 0)
	}

	// Create pipes for streaming
	retrieveReader, retrieveWriter := io.Pipe()
	decompressReader, decompressWriter := io.Pipe()

	var retrieveErr, decompressErr, restoreErr error

	logger.InfoS("Starting restore pipeline: retrieve -> decompress -> restore",
		"component", "restore",
		"target", target.Name)

	// Start retrieval in goroutine
	go func() {
		defer func() {
			if err := retrieveWriter.Close(); err != nil {
				logger.WarnS("Failed to close retrieve writer",
					"component", "restore",
					"target", target.Name,
					"error", err)
			}
		}()
		logger.InfoS("Retrieving backup from storage",
			"component", "restore",
			"target", target.Name)
		retrieveErr = stor.Retrieve(ctx, backupPath, retrieveWriter)
		if retrieveErr != nil {
			logger.ErrorS("Retrieve error",
				"component", "restore",
				"target", target.Name,
				"error", retrieveErr)
		} else {
			logger.InfoS("Backup retrieval completed",
				"component", "restore",
				"target", target.Name)
		}
	}()

	// Start DECOMPRESSING stage
	stageTracker.StartStage("DECOMPRESSING")
	if progress != nil {
		progress.SetStage("decompressing", 0)
	}

	// Start decompression in goroutine
	go func() {
		defer func() {
			if err := decompressWriter.Close(); err != nil {
				logger.WarnS("Failed to close decompress writer",
					"component", "restore",
					"target", target.Name,
					"error", err)
			}
		}()
		logger.InfoS("Starting decompression",
			"component", "restore",
			"target", target.Name)
		decompressor, err := compress.New("tgz", target.Conn.Database)
		if err != nil {
			decompressErr = err
			return
		}
		decompressErr = decompressor.Decompress(ctx, retrieveReader, decompressWriter)
		if decompressErr != nil {
			logger.ErrorS("Decompress error",
				"component", "restore",
				"target", target.Name,
				"error", decompressErr)
		} else {
			logger.InfoS("Decompression completed",
				"component", "restore",
				"target", target.Name)
		}
	}()

	// Start RESTORING stage
	stageTracker.StartStage("RESTORING")
	if progress != nil {
		progress.SetStage("restoring", 0)
	}

	// Restore from decompressed data (this blocks until all data is read)
	logger.InfoS("Starting database restore",
		"component", "restore",
		"target", target.Name)
	restoreErr = restorer.Restore(ctx, decompressReader)
	if restoreErr != nil {
		stageTracker.FailStage(restoreErr)
		return fmt.Errorf("restore failed: %w", restoreErr)
	}
	logger.InfoS("Database restore completed",
		"component", "restore",
		"target", target.Name)

	// Check for errors in pipeline
	if retrieveErr != nil {
		stageTracker.FailStage(retrieveErr)
		return fmt.Errorf("retrieve failed: %w", retrieveErr)
	}
	if decompressErr != nil {
		stageTracker.FailStage(decompressErr)
		return fmt.Errorf("decompress failed: %w", decompressErr)
	}

	// End RESTORING stage
	stageTracker.EndStage(nil)

	return nil
}

// restoreWithoutDecompression performs restore without decompression
func restoreWithoutDecompression(ctx context.Context, target *config.Target, restorer database.Restorer, stor storage.Storage, backupPath string, stageTracker *util.StageTracker, progress Progress) error {
	logger := util.GetLogger()

	// Start RETRIEVING stage
	stageTracker.StartStage("RETRIEVING")
	if progress != nil {
		progress.SetStage("retrieving", 0)
	}

	// Create pipe for streaming
	reader, writer := io.Pipe()

	var retrieveErr, restoreErr error

	logger.InfoS("Starting restore pipeline: retrieve -> restore",
		"component", "restore",
		"target", target.Name)

	// Start retrieval in goroutine
	go func() {
		defer func() {
			if err := writer.Close(); err != nil {
				logger.WarnS("Failed to close writer",
					"component", "restore",
					"target", target.Name,
					"error", err)
			}
		}()
		logger.InfoS("Retrieving backup from storage",
			"component", "restore",
			"target", target.Name)
		retrieveErr = stor.Retrieve(ctx, backupPath, writer)
		if retrieveErr == nil {
			logger.InfoS("Backup retrieval completed",
				"component", "restore",
				"target", target.Name)
		}
	}()

	// Start RESTORING stage
	stageTracker.StartStage("RESTORING")
	if progress != nil {
		progress.SetStage("restoring", 0)
	}

	// Restore data (this blocks until all data is read)
	logger.InfoS("Starting database restore",
		"component", "restore",
		"target", target.Name)
	restoreErr = restorer.Restore(ctx, reader)
	if restoreErr == nil {
		logger.InfoS("Database restore completed",
			"component", "restore",
			"target", target.Name)
	}

	// Check for errors
	if retrieveErr != nil {
		stageTracker.FailStage(retrieveErr)
		return fmt.Errorf("retrieve failed: %w", retrieveErr)
	}
	if restoreErr != nil {
		stageTracker.FailStage(restoreErr)
		return fmt.Errorf("restore failed: %w", restoreErr)
	}

	// End RESTORING stage
	stageTracker.EndStage(nil)

	return nil
}

// sendRestoreNotifications sends notifications to all configured notifiers for restore operations
func sendRestoreNotifications(ctx context.Context, cfg *config.Config, target *config.Target, result *RestoreResult, err error) {
	logger := util.GetLogger()

	if cfg == nil || len(cfg.Notifiers) == 0 {
		return
	}

	// Build notification message with all available metrics
	msg := &notify.Message{
		// Basic information
		Target:    target.Name,
		Operation: "restore",
		Duration:  result.Duration,
		Error:     err,
		Timestamp: time.Now(),

		// File path and size metrics
		Path: result.BackupPath,
		Size: result.BackupSize,

		// Storage details
		StorageName: result.StorageName,
		StorageType: result.StorageType,

		// Database details
		DatabaseType: result.DatabaseType,
		DatabaseName: result.DatabaseName,

		// Operation context
		Manual:      result.Manual,
		ScheduledBy: result.ScheduledBy,
		DryRun:      result.DryRun,

		// Restore-specific
		Validations:       result.Validations,
		ValidationsPassed: result.ValidationsPassed,
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
			logger.ErrorS("Failed to create notifier",
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
				"destination", dest,
				"timestamp", msg.Timestamp.Format(time.RFC3339))
			if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
				logger.ErrorS("Failed to send failure notification",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "failure",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"timestamp", msg.Timestamp.Format(time.RFC3339),
					"duration", time.Since(start),
					"error", sendErr)
			} else {
				logger.InfoS("Sent failure notification",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "failure",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"timestamp", msg.Timestamp.Format(time.RFC3339),
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
				"destination", dest,
				"timestamp", msg.Timestamp.Format(time.RFC3339))
			if sendErr := notifier.NotifySuccess(ctx, msg); sendErr != nil {
				logger.ErrorS("Failed to send success notification",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "success",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"timestamp", msg.Timestamp.Format(time.RFC3339),
					"duration", time.Since(start),
					"error", sendErr)
			} else {
				logger.InfoS("Sent success notification",
					"component", "notifier",
					"operation", msg.Operation,
					"status", "success",
					"target", msg.Target,
					"notifier", notifierName,
					"type", notifierCfg.Type,
					"destination", dest,
					"timestamp", msg.Timestamp.Format(time.RFC3339),
					"duration", time.Since(start))
			}
		} else {
			// Avoid silent "success but no notification" confusion when on_success is disabled.
			logger.DebugS("Skipping success notification (on_success disabled)",
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
