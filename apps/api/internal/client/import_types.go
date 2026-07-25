// Package client provides HTTP client utilities for config import operations.
package client

// ConflictMode represents how to handle conflicts during import
type ConflictMode string

const (
	// ModeInteractive prompts user for each conflict
	ModeInteractive ConflictMode = "interactive"
	// ModeOverride updates all existing configurations
	ModeOverride ConflictMode = "override"
	// ModeSkip skips existing configurations, only creates new ones
	ModeSkip ConflictMode = "skip"
)

// ConflictAction represents the user's choice for a conflict in interactive mode
type ConflictAction string

const (
	// ActionUpdate overwrites the existing configuration
	ActionUpdate ConflictAction = "update"
	// ActionSkip keeps the existing configuration
	ActionSkip ConflictAction = "skip"
	// ActionAbort cancels the entire import operation
	ActionAbort ConflictAction = "abort"
)

// ImportSummary contains the results of an import operation
type ImportSummary struct {
	Storages       ResourceSummary
	Notifiers      ResourceSummary
	Targets        ResourceSummary
	RestoreTargets ResourceSummary
	GlobalConfig   GlobalConfigSummary
	DryRun         bool
}

// ResourceSummary tracks the outcome for a resource type
type ResourceSummary struct {
	Created []string
	Updated []string
	Skipped []string
	Failed  []FailedResource
}

// FailedResource represents a resource that failed to import
type FailedResource struct {
	Name  string
	Error string
}

// GlobalConfigSummary tracks global config updates
type GlobalConfigSummary struct {
	Updated []string
	Skipped []string
	Failed  []FailedConfig
}

// FailedConfig represents a global config key that failed to update
type FailedConfig struct {
	Key   string
	Error string
}

// HasErrors returns true if any resources failed to import
func (s *ImportSummary) HasErrors() bool {
	return len(s.Storages.Failed) > 0 ||
		len(s.Notifiers.Failed) > 0 ||
		len(s.Targets.Failed) > 0 ||
		len(s.RestoreTargets.Failed) > 0 ||
		len(s.GlobalConfig.Failed) > 0
}

// TotalCreated returns the total number of resources created
func (s *ImportSummary) TotalCreated() int {
	return len(s.Storages.Created) +
		len(s.Notifiers.Created) +
		len(s.Targets.Created) +
		len(s.RestoreTargets.Created)
}

// TotalUpdated returns the total number of resources updated
func (s *ImportSummary) TotalUpdated() int {
	return len(s.Storages.Updated) +
		len(s.Notifiers.Updated) +
		len(s.Targets.Updated) +
		len(s.RestoreTargets.Updated) +
		len(s.GlobalConfig.Updated)
}

// TotalSkipped returns the total number of resources skipped
func (s *ImportSummary) TotalSkipped() int {
	return len(s.Storages.Skipped) +
		len(s.Notifiers.Skipped) +
		len(s.Targets.Skipped) +
		len(s.RestoreTargets.Skipped) +
		len(s.GlobalConfig.Skipped)
}

// TotalFailed returns the total number of resources that failed
func (s *ImportSummary) TotalFailed() int {
	return len(s.Storages.Failed) +
		len(s.Notifiers.Failed) +
		len(s.Targets.Failed) +
		len(s.RestoreTargets.Failed) +
		len(s.GlobalConfig.Failed)
}
