package configservice

import (
	"context"
	"fmt"

	"bared/internal/config"
)

// Loader handles loading configuration from database or YAML fallback
type Loader struct {
	service    *Service
	yamlConfig *config.Config
}

// NewLoader creates a new config loader
func NewLoader(service *Service, yamlConfig *config.Config) *Loader {
	return &Loader{
		service:    service,
		yamlConfig: yamlConfig,
	}
}

// LoadConfig loads configuration with DB-first fallback to YAML
func (l *Loader) LoadConfig(ctx context.Context) (*config.Config, ConfigSource, error) {
	// Check if DB has configs
	hasDBConfigs, err := l.service.HasDatabaseConfigs(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check for database configs: %w", err)
	}

	if hasDBConfigs {
		// Load from DB
		cfg, err := l.loadFromDB(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load from database: %w", err)
		}
		return cfg, SourceDatabase, nil
	}

	// Fall back to YAML
	if l.yamlConfig != nil {
		return l.yamlConfig, SourceYAML, nil
	}

	// No configs available
	return &config.Config{
		Storages:       make(map[string]*config.Storage),
		Notifiers:      make(map[string]*config.Notifier),
		Targets:        []*config.Target{},
		RestoreTargets: []*config.RestoreTarget{},
	}, SourceDatabase, nil
}

// loadFromDB loads complete config from database
func (l *Loader) loadFromDB(ctx context.Context) (*config.Config, error) {
	cfg := &config.Config{}

	// Load storages
	storages, err := l.service.ListStorages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load storages: %w", err)
	}
	cfg.Storages = storages

	// Load notifiers
	notifiers, err := l.service.ListNotifiers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load notifiers: %w", err)
	}
	cfg.Notifiers = notifiers

	// Load targets
	targets, err := l.service.ListTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load targets: %w", err)
	}
	cfg.Targets = targets

	// Load restore targets
	restoreTargets, err := l.service.ListRestoreTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load restore_targets: %w", err)
	}
	cfg.RestoreTargets = restoreTargets

	// Load global config
	globalConfigs, err := l.service.ListGlobalConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	if defaultStorage, ok := globalConfigs["default_storage"]; ok {
		cfg.DefaultStorage = defaultStorage
	}
	if logLevel, ok := globalConfigs["log_level"]; ok {
		cfg.LogLevel = logLevel
	}
	if logFormat, ok := globalConfigs["log_format"]; ok {
		cfg.LogFormat = logFormat
	}

	// Persistence is not stored in DB - always use runtime persistence config
	// This allows the daemon to control where it stores configs
	if l.yamlConfig != nil && l.yamlConfig.Persistence != nil {
		cfg.Persistence = l.yamlConfig.Persistence
	}

	return cfg, nil
}

// GetConfigSource returns the current config source
func (l *Loader) GetConfigSource(ctx context.Context) (ConfigSource, error) {
	hasDBConfigs, err := l.service.HasDatabaseConfigs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check for database configs: %w", err)
	}

	if hasDBConfigs {
		return SourceDatabase, nil
	}

	return SourceYAML, nil
}
