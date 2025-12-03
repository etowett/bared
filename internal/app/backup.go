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

	log.Printf("[%s] Backup path: %s", target.Name, backupPath)

	// Execute backup pipeline
	var dumpMetadata *database.DumpMetadata
	if target.Compress != nil && target.Compress.Enabled {
		dumpMetadata, err = backupWithCompression(ctx, target, dumper, stor, backupPath)
	} else {
		dumpMetadata, err = backupWithoutCompression(ctx, dumper, stor, backupPath)
	}

	if err != nil {
		result.Error = fmt.Errorf("backup failed: %w", err)
		return result, result.Error
	}

	// Populate result with metadata
	if dumpMetadata != nil {
		result.Size = dumpMetadata.Size
		// StoragePath is already set in result.BackupPath
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
	// Create pipes for streaming
	dumpReader, dumpWriter := io.Pipe()
	compressReader, compressWriter := io.Pipe()

	var dumpErr, compressErr, storeErr error
	var dumpMetadata *database.DumpMetadata

	// Wrap compressWriter to count bytes (final size after compression)
	counter := &countingWriter{w: compressWriter}

	// Start dump in goroutine
	go func() {
		defer dumpWriter.Close()
		meta, err := dumper.Dump(ctx, dumpWriter)
		if err != nil {
			dumpErr = err
			log.Printf("Dump error: %v", err)
		} else {
			dumpMetadata = meta
		}
	}()

	// Start compression in goroutine
	go func() {
		defer compressWriter.Close()
		compressor, err := compress.New(target.Compress.Type, target.Conn.Database)
		if err != nil {
			compressErr = err
			return
		}
		compressErr = compressor.Compress(ctx, dumpReader, counter)
		if compressErr != nil {
			log.Printf("Compress error: %v", compressErr)
		}
	}()

	// Store compressed data (this blocks until all data is written)
	storeErr = stor.Store(ctx, backupPath, compressReader, -1)
	if storeErr != nil {
		return nil, fmt.Errorf("store failed: %w", storeErr)
	}

	// Check for errors in pipeline
	if dumpErr != nil {
		return nil, fmt.Errorf("dump failed: %w", dumpErr)
	}
	if compressErr != nil {
		return nil, fmt.Errorf("compress failed: %w", compressErr)
	}

	// Populate metadata with file information
	if dumpMetadata == nil {
		dumpMetadata = &database.DumpMetadata{}
	}
	dumpMetadata.Size = counter.Size()
	dumpMetadata.StoragePath = backupPath

	return dumpMetadata, nil
}

// backupWithoutCompression performs backup without compression
func backupWithoutCompression(ctx context.Context, dumper database.Dumper, stor storage.Storage, backupPath string) (*database.DumpMetadata, error) {
	// Create pipe for streaming
	reader, writer := io.Pipe()

	var dumpErr, storeErr error
	var dumpMetadata *database.DumpMetadata

	// Wrap writer to count bytes (final size)
	counter := &countingWriter{w: writer}

	// Start dump in goroutine
	go func() {
		defer writer.Close()
		meta, err := dumper.Dump(ctx, counter)
		if err != nil {
			dumpErr = err
		} else {
			dumpMetadata = meta
		}
	}()

	// Store data (this blocks until all data is written)
	storeErr = stor.Store(ctx, backupPath, reader, -1)

	// Check for errors
	if dumpErr != nil {
		return nil, fmt.Errorf("dump failed: %w", dumpErr)
	}
	if storeErr != nil {
		return nil, fmt.Errorf("store failed: %w", storeErr)
	}

	// Populate metadata with file information
	if dumpMetadata == nil {
		dumpMetadata = &database.DumpMetadata{}
	}
	dumpMetadata.Size = counter.Size()
	dumpMetadata.StoragePath = backupPath

	return dumpMetadata, nil
}
