package config

// Config represents the entire configuration file
type Config struct {
	DefaultStorage string               `yaml:"default_storage"`
	LogLevel       string               `yaml:"log_level,omitempty"`
	Storages       map[string]*Storage  `yaml:"storages"`
	Notifiers      map[string]*Notifier `yaml:"notifiers"`
	Targets        []*Target            `yaml:"targets"`
	RestoreTargets []*RestoreTarget     `yaml:"restore_targets,omitempty"`
}

// Storage configuration
type Storage struct {
	Type            string `yaml:"type"`
	Name            string `yaml:"name"`
	Path            string `yaml:"path,omitempty"`
	Keep            int    `yaml:"keep"`
	Bucket          string `yaml:"bucket,omitempty"`
	Region          string `yaml:"region,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	EndpointURL     string `yaml:"endpoint_url,omitempty"`
	Host            string `yaml:"host,omitempty"`
	Port            int    `yaml:"port,omitempty"`
	Username        string `yaml:"username,omitempty"`
	Password        string `yaml:"password,omitempty"`
}

// Notifier configuration
type Notifier struct {
	Type      string `yaml:"type"`
	URL       string `yaml:"url"`
	OnSuccess bool   `yaml:"on_success"`
}

// Target represents a backup target (database)
type Target struct {
	Name           string           `yaml:"name"`
	Conn           *Connection      `yaml:"conn"`
	ExcludeTables  []string         `yaml:"exclude_tables,omitempty"`
	AdditionalArgs []string         `yaml:"additional_args,omitempty"`
	Compress       *CompressionOpts `yaml:"compress,omitempty"`
	Storage        *TargetStorage   `yaml:"storage,omitempty"`
	Schedule       string           `yaml:"schedule,omitempty"`
}

// Connection details for a database
type Connection struct {
	Type     string `yaml:"type"`
	User     string `yaml:"user,omitempty"`
	Password string `yaml:"password,omitempty"`
	Database string `yaml:"database,omitempty"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
}

// CompressionOpts defines compression settings
type CompressionOpts struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"`
}

// TargetStorage links a target to a storage backend
type TargetStorage struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
}

// RestoreTarget represents a restore destination (can differ from backup source)
type RestoreTarget struct {
	Name         string          `yaml:"name"`
	Conn         *Connection     `yaml:"conn"`
	SourceTarget string          `yaml:"source_target,omitempty"`
	Storage      *TargetStorage  `yaml:"storage,omitempty"`
	Description  string          `yaml:"description,omitempty"`
}
