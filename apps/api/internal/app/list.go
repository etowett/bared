package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/storage"
	"github.com/etowett/bared/apps/api/internal/util"
)

// ListBackups lists all backups for a target
func ListBackups(ctx context.Context, cfg *config.Config, target *config.Target) ([]*storage.BackupInfo, error) {
	logger := util.GetLogger()

	// Get storage backend
	storageCfg, err := cfg.GetStorageForTarget(target)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage: %w", err)
	}

	stor, err := storage.New(storageCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	logger.InfoS("Listing backups",
		"component", "app",
		"target", target.Name)

	// List all backups
	allBackups, err := stor.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	// Filter backups for this target
	var targetBackups []*storage.BackupInfo
	targetPrefix := target.Name + "/"

	for _, backup := range allBackups {
		if strings.HasPrefix(backup.Path, targetPrefix) {
			targetBackups = append(targetBackups, backup)
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(targetBackups, func(i, j int) bool {
		return targetBackups[i].LastModified.After(targetBackups[j].LastModified)
	})

	return targetBackups, nil
}

// FindLatestBackup finds the most recent backup for a target
func FindLatestBackup(ctx context.Context, cfg *config.Config, target *config.Target) (*storage.BackupInfo, error) {
	backups, err := ListBackups(ctx, cfg, target)
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no backups found for target '%s'", target.Name)
	}

	// Backups are already sorted by newest first
	return backups[0], nil
}
