package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	//#nosec G304 -- path is from user-provided config file argument
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedData := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// expandEnvVars replaces ${VAR_NAME} with environment variable values
func expandEnvVars(data string) string {
	return envVarRegex.ReplaceAllStringFunc(data, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := match[2 : len(match)-1]

		// Get environment variable value
		if value := os.Getenv(varName); value != "" {
			return value
		}

		// Return original if not found (will likely fail validation)
		return match
	})
}

// FindTarget finds a target by name
func (c *Config) FindTarget(name string) (*Target, error) {
	for _, target := range c.Targets {
		if target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target '%s' not found in configuration", name)
}

// FindStorage finds a storage by name
func (c *Config) FindStorage(name string) (*Storage, error) {
	storage, ok := c.Storages[name]
	if !ok {
		return nil, fmt.Errorf("storage '%s' not found in configuration", name)
	}
	return storage, nil
}

// FindNotifier finds a notifier by name
func (c *Config) FindNotifier(name string) (*Notifier, error) {
	notifier, ok := c.Notifiers[name]
	if !ok {
		return nil, fmt.Errorf("notifier '%s' not found in configuration", name)
	}
	return notifier, nil
}

// GetStorageForTarget returns the storage backend for a target
func (c *Config) GetStorageForTarget(target *Target) (*Storage, error) {
	// Use target-specific storage if configured
	if target.Storage != nil && target.Storage.Enabled {
		return c.FindStorage(target.Storage.Name)
	}

	// Fall back to default storage
	if c.DefaultStorage != "" {
		return c.FindStorage(c.DefaultStorage)
	}

	return nil, fmt.Errorf("no storage configured for target '%s'", target.Name)
}

// GetAllNotifiers returns all configured notifiers
func (c *Config) GetAllNotifiers() []*Notifier {
	notifiers := make([]*Notifier, 0, len(c.Notifiers))
	for _, notifier := range c.Notifiers {
		notifiers = append(notifiers, notifier)
	}
	return notifiers
}

// FindRestoreTarget finds a restore target by name
func (c *Config) FindRestoreTarget(name string) (*RestoreTarget, error) {
	for _, rt := range c.RestoreTargets {
		if rt.Name == name {
			return rt, nil
		}
	}
	return nil, fmt.Errorf("restore target '%s' not found in configuration", name)
}

// GetStorageForRestoreTarget returns the storage backend for a restore target
func (c *Config) GetStorageForRestoreTarget(restoreTarget *RestoreTarget) (*Storage, error) {
	// Use restore target's storage if configured
	if restoreTarget.Storage != nil && restoreTarget.Storage.Enabled {
		return c.FindStorage(restoreTarget.Storage.Name)
	}

	// Fall back to source target's storage
	if restoreTarget.SourceTarget != "" {
		sourceTarget, err := c.FindTarget(restoreTarget.SourceTarget)
		if err != nil {
			return nil, fmt.Errorf("source target '%s' not found", restoreTarget.SourceTarget)
		}
		return c.GetStorageForTarget(sourceTarget)
	}

	// Fall back to default storage
	if c.DefaultStorage != "" {
		return c.FindStorage(c.DefaultStorage)
	}

	return nil, fmt.Errorf("no storage configured for restore target '%s'", restoreTarget.Name)
}

// ResolveRestoreTarget resolves either a regular target or restore target by name
// Returns (Target, RestoreTarget, isRestoreTarget, error)
func (c *Config) ResolveRestoreTarget(name string) (*Target, *RestoreTarget, bool, error) {
	// First try to find as restore target
	rt, err := c.FindRestoreTarget(name)
	if err == nil {
		// Convert RestoreTarget to Target for restore operations
		target := &Target{
			Name:    rt.Name,
			Conn:    rt.Conn,
			Storage: rt.Storage,
		}
		return target, rt, true, nil
	}

	// Fall back to regular target
	target, err := c.FindTarget(name)
	if err != nil {
		return nil, nil, false, fmt.Errorf("target or restore target '%s' not found", name)
	}

	return target, nil, false, nil
}
