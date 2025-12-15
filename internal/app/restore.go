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
	Target      string
	Success     bool
	Error       error
	Duration    time.Duration
	BackupPath  string

	// Storage details
	StorageName string
	StorageType string
	BackupSize  int64

	// Validation details
	DryRun            bool
	Validations       []string
	ValidationsPassed int

	// Database details
	DatabaseType string
	DatabaseName string
	RestoredBytes int64

	// Operation context
	Manual      bool
	ScheduledBy string

	// Stage information
	Stages []*util.Stage
}

// RestoreTargetWithOptions performs a restore with specified options
func RestoreTargetWithOptions(ctx context.Context, cfg *config.Config, target *config.Target, backupPath string, options *RestoreOptions, progress Progress) (*RestoreResult, error) {
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

	dryRunSuffix := ""
	if result.DryRun {
		dryRunSuffix = " (DRY-RUN)"
	}
	log.Printf("[%s] Starting restore%s from: %s", target.Name, dryRunSuffix, backupPath)

	// Step 1: Create database restorer
	restorer, err := database.NewRestorer(target)
	if err != nil {
		result.Error = fmt.Errorf("failed to create restorer: %w", err)
		return result, result.Error
	}
	result.Validations = append(result.Validations, "Database restorer created")

	// Step 2: Validate database connection (unless skipped)
	if !options.SkipValidation {
		log.Printf("[%s] Validating database connection...", target.Name)
		if connErr := restorer.ValidateConnection(ctx); connErr != nil {
			result.Error = fmt.Errorf("database connection validation failed: %w", connErr)
			return result, result.Error
		}
		result.Validations = append(result.Validations, "Database connection validated")
		log.Printf("[%s] Database connection validated successfully", target.Name)
	}

	// Step 3: Get storage backend
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

	// Set storage details
	result.StorageName = stor.Name()
	result.StorageType = storageCfg.Type
	result.Validations = append(result.Validations, fmt.Sprintf("Storage backend connected: %s", stor.Name()))

	// Set database details
	result.DatabaseType = target.Conn.Type
	result.DatabaseName = target.Conn.Database

	// Step 4: Validate storage
	if err := stor.Validate(ctx); err != nil {
		result.Error = fmt.Errorf("storage validation failed: %w", err)
		return result, result.Error
	}
	result.Validations = append(result.Validations, "Storage validated")

	// Step 5: Verify backup file exists (unless skipped)
	if !options.SkipBackupVerify {
		log.Printf("[%s] Verifying backup file exists...", target.Name)
		exists, err := stor.Exists(ctx, backupPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to verify backup existence: %w", err)
			return result, result.Error
		}
		if !exists {
			result.Error = fmt.Errorf("backup file not found: %s", backupPath)
			return result, result.Error
		}

		// Get backup file info
		backupInfo, err := stor.GetInfo(ctx, backupPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to get backup info: %w", err)
			return result, result.Error
		}

		// Store backup size
		result.BackupSize = backupInfo.Size

		result.Validations = append(result.Validations,
			fmt.Sprintf("Backup file verified: %s (size: %d bytes, modified: %s)",
				backupPath, backupInfo.Size, backupInfo.LastModified.Format(time.RFC3339)))
		log.Printf("[%s] Backup file verified: %d bytes", target.Name, backupInfo.Size)
	}

	// Step 6: Check if backup needs decompression
	needsDecompression := strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz")
	if needsDecompression {
		result.Validations = append(result.Validations, "Backup requires decompression")
		log.Printf("[%s] Backup requires decompression", target.Name)
	}

	// End validation stage
	stageTracker.EndStage(map[string]interface{}{
		"validations_passed": len(result.Validations),
		"needs_decompression": needsDecompression,
	})

	// Store validations count
	result.ValidationsPassed = len(result.Validations)

	// If dry-run, stop here
	if result.DryRun {
		result.Success = true
		result.Duration = time.Since(startTime)
		result.Stages = stageTracker.GetAllStages()
		log.Printf("[%s] DRY-RUN completed successfully. All validations passed.", target.Name)
		log.Printf("[%s] Validations performed:", target.Name)
		for _, v := range result.Validations {
			log.Printf("[%s]   - %s", target.Name, v)
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
		result.Error = fmt.Errorf("restore failed: %w", restoreErr)
		result.Stages = stageTracker.GetAllStages()
		log.Printf("[%s] Restore failed: %v", target.Name, restoreErr)

		// Send failure notifications
		sendRestoreNotifications(ctx, cfg, target, result, restoreErr)

		return result, result.Error
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	// Store stage information
	result.Stages = stageTracker.GetAllStages()

	log.Printf("[%s] Restore completed successfully in %v", target.Name, result.Duration)

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
	// Start RETRIEVING stage
	stageTracker.StartStage("RETRIEVING")
	if progress != nil {
		progress.SetStage("retrieving", 0)
	}

	// Create pipes for streaming
	retrieveReader, retrieveWriter := io.Pipe()
	decompressReader, decompressWriter := io.Pipe()

	var retrieveErr, decompressErr, restoreErr error

	log.Printf("[%s] Starting restore pipeline: retrieve -> decompress -> restore", target.Name)

	// Start retrieval in goroutine
	go func() {
		defer func() {
			if err := retrieveWriter.Close(); err != nil {
				log.Printf("[%s] Failed to close retrieve writer: %v", target.Name, err)
			}
		}()
		log.Printf("[%s] Retrieving backup from storage", target.Name)
		retrieveErr = stor.Retrieve(ctx, backupPath, retrieveWriter)
		if retrieveErr != nil {
			log.Printf("[%s] Retrieve error: %v", target.Name, retrieveErr)
		} else {
			log.Printf("[%s] Backup retrieval completed", target.Name)
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
				log.Printf("[%s] Failed to close decompress writer: %v", target.Name, err)
			}
		}()
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

	// Start RESTORING stage
	stageTracker.StartStage("RESTORING")
	if progress != nil {
		progress.SetStage("restoring", 0)
	}

	// Restore from decompressed data (this blocks until all data is read)
	log.Printf("[%s] Starting database restore", target.Name)
	restoreErr = restorer.Restore(ctx, decompressReader)
	if restoreErr != nil {
		stageTracker.FailStage(restoreErr)
		return fmt.Errorf("restore failed: %w", restoreErr)
	}
	log.Printf("[%s] Database restore completed", target.Name)

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
	// Start RETRIEVING stage
	stageTracker.StartStage("RETRIEVING")
	if progress != nil {
		progress.SetStage("retrieving", 0)
	}

	// Create pipe for streaming
	reader, writer := io.Pipe()

	var retrieveErr, restoreErr error

	log.Printf("[%s] Starting restore pipeline: retrieve -> restore", target.Name)

	// Start retrieval in goroutine
	go func() {
		defer func() {
			if err := writer.Close(); err != nil {
				log.Printf("[%s] Failed to close writer: %v", target.Name, err)
			}
		}()
		log.Printf("[%s] Retrieving backup from storage", target.Name)
		retrieveErr = stor.Retrieve(ctx, backupPath, writer)
		if retrieveErr == nil {
			log.Printf("[%s] Backup retrieval completed", target.Name)
		}
	}()

	// Start RESTORING stage
	stageTracker.StartStage("RESTORING")
	if progress != nil {
		progress.SetStage("restoring", 0)
	}

	// Restore data (this blocks until all data is read)
	log.Printf("[%s] Starting database restore", target.Name)
	restoreErr = restorer.Restore(ctx, reader)
	if restoreErr == nil {
		log.Printf("[%s] Database restore completed", target.Name)
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
	notifiers := cfg.GetAllNotifiers()
	if len(notifiers) == 0 {
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

	for _, notifierCfg := range notifiers {
		notifier, notifyErr := notify.New(notifierCfg)
		if notifyErr != nil {
			log.Printf("Warning: failed to create notifier: %v", notifyErr)
			continue
		}

		// Send success or failure notification
		if err != nil {
			if sendErr := notifier.NotifyFailure(ctx, msg); sendErr != nil {
				log.Printf("Warning: failed to send restore failure notification: %v", sendErr)
			}
		} else if notifier.ShouldNotifySuccess() {
			if sendErr := notifier.NotifySuccess(ctx, msg); sendErr != nil {
				log.Printf("Warning: failed to send restore success notification: %v", sendErr)
			}
		}
	}
}
