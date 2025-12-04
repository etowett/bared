package config

import (
	"fmt"
	"strings"
)

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	var errors []string

	// Validate storages
	if len(c.Storages) == 0 {
		errors = append(errors, "at least one storage backend must be configured")
	}

	for name, storage := range c.Storages {
		if err := validateStorage(name, storage); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate targets
	if len(c.Targets) == 0 {
		errors = append(errors, "at least one target must be configured")
	}

	targetNames := make(map[string]bool)
	for _, target := range c.Targets {
		// Check for duplicate names
		if targetNames[target.Name] {
			errors = append(errors, fmt.Sprintf("duplicate target name: %s", target.Name))
		}
		targetNames[target.Name] = true

		if err := validateTarget(target, c); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate default storage exists
	if c.DefaultStorage != "" {
		if _, ok := c.Storages[c.DefaultStorage]; !ok {
			errors = append(errors, fmt.Sprintf("default_storage '%s' not found in storages", c.DefaultStorage))
		}
	}

	// Validate restore targets
	restoreTargetNames := make(map[string]bool)
	for _, rt := range c.RestoreTargets {
		// Check for duplicate names with regular targets
		if targetNames[rt.Name] {
			errors = append(errors, fmt.Sprintf("restore target name '%s' conflicts with regular target", rt.Name))
		}

		// Check for duplicate names among restore targets
		if restoreTargetNames[rt.Name] {
			errors = append(errors, fmt.Sprintf("duplicate restore target name: %s", rt.Name))
		}
		restoreTargetNames[rt.Name] = true

		if err := validateRestoreTarget(rt, c); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate notifiers
	for name, notifier := range c.Notifiers {
		if err := validateNotifier(name, notifier); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

func validateStorage(name string, storage *Storage) error {
	if storage.Type == "" {
		return fmt.Errorf("storage '%s': type is required", name)
	}

	// Validate keep value
	if storage.Keep < 0 {
		return fmt.Errorf("storage '%s': keep must be non-negative", name)
	}

	switch storage.Type {
	case "local":
		if storage.Path == "" {
			return fmt.Errorf("storage '%s': path is required for local storage", name)
		}
	case "s3":
		if storage.Bucket == "" {
			return fmt.Errorf("storage '%s': bucket is required for s3 storage", name)
		}
		if storage.Region == "" {
			return fmt.Errorf("storage '%s': region is required for s3 storage", name)
		}
	case "sftp":
		if storage.Host == "" {
			return fmt.Errorf("storage '%s': host is required for sftp storage", name)
		}
		if storage.Port == 0 {
			return fmt.Errorf("storage '%s': port is required for sftp storage", name)
		}
		if storage.Username == "" {
			return fmt.Errorf("storage '%s': username is required for sftp storage", name)
		}
	default:
		return fmt.Errorf("storage '%s': unsupported type '%s'", name, storage.Type)
	}

	return nil
}

func validateTarget(target *Target, cfg *Config) error {
	if target.Name == "" {
		return fmt.Errorf("target: name is required")
	}

	if target.Conn == nil {
		return fmt.Errorf("target '%s': conn is required", target.Name)
	}

	if err := validateConnection(target.Name, target.Conn); err != nil {
		return err
	}

	// Validate compression settings
	if target.Compress != nil && target.Compress.Enabled {
		if target.Compress.Type == "" {
			return fmt.Errorf("target '%s': compression type is required when compression is enabled", target.Name)
		}
		if target.Compress.Type != "tgz" {
			return fmt.Errorf("target '%s': unsupported compression type '%s'", target.Name, target.Compress.Type)
		}
	}

	// Validate storage reference
	if target.Storage != nil && target.Storage.Enabled {
		if target.Storage.Name == "" {
			return fmt.Errorf("target '%s': storage name is required when storage is enabled", target.Name)
		}
		if _, ok := cfg.Storages[target.Storage.Name]; !ok {
			return fmt.Errorf("target '%s': storage '%s' not found", target.Name, target.Storage.Name)
		}
	} else if cfg.DefaultStorage == "" {
		return fmt.Errorf("target '%s': no storage configured and no default_storage set", target.Name)
	}

	return nil
}

func validateConnection(targetName string, conn *Connection) error {
	if conn.Type == "" {
		return fmt.Errorf("target '%s': connection type is required", targetName)
	}

	// Validate port range for all connection types
	if conn.Port != 0 && (conn.Port < 1 || conn.Port > 65535) {
		return fmt.Errorf("target '%s': port must be between 1 and 65535", targetName)
	}

	switch conn.Type {
	case "mysql", "postgres":
		if conn.Host == "" {
			return fmt.Errorf("target '%s': host is required for %s", targetName, conn.Type)
		}
		if conn.Port == 0 {
			return fmt.Errorf("target '%s': port is required for %s", targetName, conn.Type)
		}
		if conn.User == "" {
			return fmt.Errorf("target '%s': user is required for %s", targetName, conn.Type)
		}
		if conn.Database == "" {
			return fmt.Errorf("target '%s': database is required for %s", targetName, conn.Type)
		}
	case "redis":
		if conn.Host == "" {
			return fmt.Errorf("target '%s': host is required for redis", targetName)
		}
		if conn.Port == 0 {
			return fmt.Errorf("target '%s': port is required for redis", targetName)
		}
	default:
		return fmt.Errorf("target '%s': unsupported database type '%s'", targetName, conn.Type)
	}

	return nil
}

func validateNotifier(name string, notifier *Notifier) error {
	if notifier.Type == "" {
		return fmt.Errorf("notifier '%s': type is required", name)
	}

	switch notifier.Type {
	case "slack":
		if notifier.URL == "" {
			return fmt.Errorf("notifier '%s': url is required for slack", name)
		}
	default:
		return fmt.Errorf("notifier '%s': unsupported type '%s'", name, notifier.Type)
	}

	return nil
}

func validateRestoreTarget(rt *RestoreTarget, cfg *Config) error {
	if rt.Name == "" {
		return fmt.Errorf("restore target: name is required")
	}

	if rt.Conn == nil {
		return fmt.Errorf("restore target '%s': conn is required", rt.Name)
	}

	if err := validateConnection(rt.Name, rt.Conn); err != nil {
		return err
	}

	// Validate source target reference if provided
	if rt.SourceTarget != "" {
		if _, err := cfg.FindTarget(rt.SourceTarget); err != nil {
			return fmt.Errorf("restore target '%s': source target '%s' not found", rt.Name, rt.SourceTarget)
		}
	}

	// Validate storage reference
	if rt.Storage != nil && rt.Storage.Enabled {
		if rt.Storage.Name == "" {
			return fmt.Errorf("restore target '%s': storage name is required when storage is enabled", rt.Name)
		}
		if _, ok := cfg.Storages[rt.Storage.Name]; !ok {
			return fmt.Errorf("restore target '%s': storage '%s' not found", rt.Name, rt.Storage.Name)
		}
	} else if rt.SourceTarget == "" && cfg.DefaultStorage == "" {
		return fmt.Errorf("restore target '%s': no storage configured, no source target, and no default_storage set", rt.Name)
	}

	return nil
}
