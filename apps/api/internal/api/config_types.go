package api

// Config management request/response types

// StorageRequest represents a request to create/update storage
type StorageRequest struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"` // local, s3, sftp
	Keep            int                    `json:"keep"`
	Config          map[string]interface{} `json:"config"` // Type-specific fields
	SecretAccessKey string                 `json:"secret_access_key,omitempty"`
	Password        string                 `json:"password,omitempty"`
}

// StorageResponse represents storage in API responses
type StorageResponse struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Keep      int                    `json:"keep"`
	Config    map[string]interface{} `json:"config"` // Secrets filtered out
	Enabled   bool                   `json:"enabled"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ListStoragesResponse represents the response for listing storages
type ListStoragesResponse struct {
	Storages []StorageResponse `json:"storages"`
	Total    int               `json:"total"`
	Source   string            `json:"source"` // "database" or "yaml"
}

// NotifierRequest represents a request to create/update notifier
type NotifierRequest struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"` // slack, email, webhook
	OnSuccess bool                   `json:"on_success"`
	Config    map[string]interface{} `json:"config"` // Type-specific fields
	// Secrets (only in create/update requests)
	SMTPPassword         string `json:"smtp_password,omitempty"`
	WebhookAuthPassword  string `json:"webhook_auth_password,omitempty"`
	WebhookAuthToken     string `json:"webhook_auth_token,omitempty"`
	WebhookAuthHeaderVal string `json:"webhook_auth_header_value,omitempty"`
}

// NotifierResponse represents notifier in API responses
type NotifierResponse struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	OnSuccess bool                   `json:"on_success"`
	Config    map[string]interface{} `json:"config"` // Secrets filtered
	Enabled   bool                   `json:"enabled"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ListNotifiersResponse represents the response for listing notifiers
type ListNotifiersResponse struct {
	Notifiers []NotifierResponse `json:"notifiers"`
	Total     int                `json:"total"`
	Source    string             `json:"source"` // "database" or "yaml"
}

// TargetRequest represents a request to create/update target
type TargetRequest struct {
	Name           string             `json:"name"`
	Connection     ConnectionRequest  `json:"connection"`
	StorageName    string             `json:"storage_name,omitempty"`
	Schedule       string             `json:"schedule,omitempty"`
	Compress       *CompressionConfig `json:"compress,omitempty"`
	ExcludeTables  []string           `json:"exclude_tables,omitempty"`
	AdditionalArgs []string           `json:"additional_args,omitempty"`
}

// ConnectionRequest represents a database connection
type ConnectionRequest struct {
	Type     string `json:"type"` // mysql, postgres, redis
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// CompressionConfig represents compression settings
type CompressionConfig struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type,omitempty"` // gzip, tgz
}

// TargetResponse represents target in API responses
type TargetResponse struct {
	Name           string                 `json:"name"`
	Connection     map[string]interface{} `json:"connection"` // Password filtered
	StorageName    string                 `json:"storage_name,omitempty"`
	Schedule       string                 `json:"schedule,omitempty"`
	Compress       *CompressionConfig     `json:"compress,omitempty"`
	ExcludeTables  []string               `json:"exclude_tables,omitempty"`
	AdditionalArgs []string               `json:"additional_args,omitempty"`
	Enabled        bool                   `json:"enabled"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

// ListTargetsConfigResponse represents the response for listing target configs
type ListTargetsConfigResponse struct {
	Targets []TargetResponse `json:"targets"`
	Total   int              `json:"total"`
	Source  string           `json:"source"` // "database" or "yaml"
}

// RestoreTargetRequest represents a request to create/update restore target
type RestoreTargetRequest struct {
	Name         string            `json:"name"`
	Connection   ConnectionRequest `json:"connection"`
	StorageName  string            `json:"storage_name,omitempty"`
	SourceTarget string            `json:"source_target,omitempty"`
	Description  string            `json:"description,omitempty"`
}

// RestoreTargetResponse represents restore target in API responses
type RestoreTargetResponse struct {
	Name         string                 `json:"name"`
	Connection   map[string]interface{} `json:"connection"` // Password filtered
	StorageName  string                 `json:"storage_name,omitempty"`
	SourceTarget string                 `json:"source_target,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Enabled      bool                   `json:"enabled"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// ListRestoreTargetsConfigResponse represents the response for listing restore target configs
type ListRestoreTargetsConfigResponse struct {
	RestoreTargets []RestoreTargetResponse `json:"restore_targets"`
	Total          int                     `json:"total"`
	Source         string                  `json:"source"` // "database" or "yaml"
}

// GlobalConfigResponse represents all global config settings
type GlobalConfigResponse struct {
	DefaultStorage string `json:"default_storage"`
	LogLevel       string `json:"log_level"`
	LogFormat      string `json:"log_format"`
	Source         string `json:"source"` // "database" or "yaml"
}

// UpdateGlobalConfigRequest represents a request to update a global config value
type UpdateGlobalConfigRequest struct {
	Value string `json:"value"`
}

// MigrateConfigResponse represents the response after migrating YAML to DB
type MigrateConfigResponse struct {
	Message        string `json:"message"`
	StoragesCount  int    `json:"storages_count"`
	NotifiersCount int    `json:"notifiers_count"`
	TargetsCount   int    `json:"targets_count"`
}

// ReloadConfigResponse represents the response after reloading config
type ReloadConfigResponse struct {
	Message string `json:"message"`
	Source  string `json:"source"` // "database" or "yaml"
}

// ConfigSourceResponse represents the current config source
type ConfigSourceResponse struct {
	Source string `json:"source"` // "database" or "yaml"
}

// UpdateScheduleRequest represents a request to update a target's schedule
type UpdateScheduleRequest struct {
	Schedule string `json:"schedule"`
}

// ConfigImportRequest represents a request to import YAML configuration
type ConfigImportRequest struct {
	YAMLContent  string `json:"yaml_content"`
	ConflictMode string `json:"conflict_mode"` // "override" or "skip"
	DryRun       bool   `json:"dry_run"`
}

// ConfigImportResponse represents the result of a config import
type ConfigImportResponse struct {
	Storages       ResourceImportSummary     `json:"storages"`
	Notifiers      ResourceImportSummary     `json:"notifiers"`
	Targets        ResourceImportSummary     `json:"targets"`
	RestoreTargets ResourceImportSummary     `json:"restore_targets"`
	GlobalConfig   GlobalConfigImportSummary `json:"global_config"`
	DryRun         bool                      `json:"dry_run"`
	TotalCreated   int                       `json:"total_created"`
	TotalUpdated   int                       `json:"total_updated"`
	TotalSkipped   int                       `json:"total_skipped"`
	TotalFailed    int                       `json:"total_failed"`
	HasErrors      bool                      `json:"has_errors"`
}

// ResourceImportSummary tracks the outcome for a resource type during import
type ResourceImportSummary struct {
	Created []string               `json:"created"`
	Updated []string               `json:"updated"`
	Skipped []string               `json:"skipped"`
	Failed  []FailedImportResource `json:"failed"`
}

// FailedImportResource represents a resource that failed to import
type FailedImportResource struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// GlobalConfigImportSummary tracks global config updates during import
type GlobalConfigImportSummary struct {
	Updated []string             `json:"updated"`
	Skipped []string             `json:"skipped"`
	Failed  []FailedImportConfig `json:"failed"`
}

// FailedImportConfig represents a global config key that failed to update
type FailedImportConfig struct {
	Key   string `json:"key"`
	Error string `json:"error"`
}
