package configservice

import (
	"context"
	"fmt"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/util"
)

// ImportFromYAML imports YAML configuration into the database
func (s *Service) ImportFromYAML(ctx context.Context, yamlConfig *config.Config) error {
	logger := util.GetLogger()

	// Check if DB already has configs
	hasConfigs, err := s.HasDatabaseConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for existing configs: %w", err)
	}

	if hasConfigs {
		logger.WarnS("Database already contains configurations", "component", "configservice")
		return fmt.Errorf("database already contains configurations - migration aborted to prevent overwriting")
	}

	logger.InfoS("Starting YAML to database migration", "component", "configservice")

	// Import storages
	if len(yamlConfig.Storages) > 0 {
		logger.InfoS("Importing storages", "count", len(yamlConfig.Storages))
		for name, storage := range yamlConfig.Storages {
			// The name is the YAML map key; the decoder does not populate the
			// struct field from it. Without this every migration of a
			// real config fails ValidateStorage with "storage name is
			// required" — see importStorages in cmd/brd/config_import.go,
			// which has done this all along.
			storage.Name = name
			if err := ValidateStorage(storage); err != nil {
				return fmt.Errorf("storage %s validation failed: %w", name, err)
			}
			if err := s.CreateStorage(ctx, storage); err != nil {
				return fmt.Errorf("failed to import storage %s: %w", name, err)
			}
			logger.InfoS("Imported storage", "name", name, "type", storage.Type)
		}
	}

	// Import notifiers
	if len(yamlConfig.Notifiers) > 0 {
		logger.InfoS("Importing notifiers", "count", len(yamlConfig.Notifiers))
		for name, notifier := range yamlConfig.Notifiers {
			if err := ValidateNotifier(notifier); err != nil {
				return fmt.Errorf("notifier %s validation failed: %w", name, err)
			}
			if err := s.CreateNotifier(ctx, name, notifier); err != nil {
				return fmt.Errorf("failed to import notifier %s: %w", name, err)
			}
			logger.InfoS("Imported notifier", "name", name, "type", notifier.Type)
		}
	}

	// Import targets
	if len(yamlConfig.Targets) > 0 {
		logger.InfoS("Importing targets", "count", len(yamlConfig.Targets))
		for _, target := range yamlConfig.Targets {
			if err := ValidateTarget(target, yamlConfig.Storages); err != nil {
				return fmt.Errorf("target %s validation failed: %w", target.Name, err)
			}
			if err := s.CreateTarget(ctx, target); err != nil {
				return fmt.Errorf("failed to import target %s: %w", target.Name, err)
			}
			logger.InfoS("Imported target", "name", target.Name, "type", target.Conn.Type)
		}
	}

	// Import restore targets
	if len(yamlConfig.RestoreTargets) > 0 {
		logger.InfoS("Importing restore targets", "count", len(yamlConfig.RestoreTargets))

		// Convert targets slice to map for validation
		targetsMap := make(map[string]*config.Target)
		for _, target := range yamlConfig.Targets {
			targetsMap[target.Name] = target
		}

		for _, rt := range yamlConfig.RestoreTargets {
			if err := ValidateRestoreTarget(rt, yamlConfig.Storages, targetsMap); err != nil {
				return fmt.Errorf("restore target %s validation failed: %w", rt.Name, err)
			}
			if err := s.CreateRestoreTarget(ctx, rt); err != nil {
				return fmt.Errorf("failed to import restore target %s: %w", rt.Name, err)
			}
			logger.InfoS("Imported restore target", "name", rt.Name, "type", rt.Conn.Type)
		}
	}

	// Import global config
	if yamlConfig.DefaultStorage != "" {
		if err := s.SetGlobalConfig(ctx, "default_storage", yamlConfig.DefaultStorage); err != nil {
			return fmt.Errorf("failed to set default_storage: %w", err)
		}
		logger.InfoS("Imported default_storage", "value", yamlConfig.DefaultStorage)
	}
	if yamlConfig.LogLevel != "" {
		if err := s.SetGlobalConfig(ctx, "log_level", yamlConfig.LogLevel); err != nil {
			return fmt.Errorf("failed to set log_level: %w", err)
		}
		logger.InfoS("Imported log_level", "value", yamlConfig.LogLevel)
	}
	if yamlConfig.LogFormat != "" {
		if err := s.SetGlobalConfig(ctx, "log_format", yamlConfig.LogFormat); err != nil {
			return fmt.Errorf("failed to set log_format: %w", err)
		}
		logger.InfoS("Imported log_format", "value", yamlConfig.LogFormat)
	}

	logger.InfoS("YAML to database migration completed successfully", "component", "configservice")
	return nil
}
