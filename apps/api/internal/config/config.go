// Package config provides configuration file parsing and validation for BareD.
package config

// Config represents the entire configuration file
type Config struct {
	DefaultStorage string               `yaml:"default_storage"`
	LogLevel       string               `yaml:"log_level,omitempty"`
	LogFormat      string               `yaml:"log_format,omitempty"`  // "json", "text", or "auto" (default)
	LogOptions     *LogOptions          `yaml:"log_options,omitempty"` // Optional logging configuration
	Storages       map[string]*Storage  `yaml:"storages"`
	Notifiers      map[string]*Notifier `yaml:"notifiers"`
	Targets        []*Target            `yaml:"targets"`
	RestoreTargets []*RestoreTarget     `yaml:"restore_targets,omitempty"`
	Persistence    *Persistence         `yaml:"persistence,omitempty"`
}

// LogOptions represents optional logging configuration
type LogOptions struct {
	AddSource  bool   `yaml:"add_source,omitempty"`  // Add file:line to logs
	TimeFormat string `yaml:"time_format,omitempty"` // Custom time format
}

// Persistence configuration
type Persistence struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"` // sqlite, postgres, mysql
	DSN     string `yaml:"dsn"`  // Data Source Name
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

	// SFTP host key verification.
	//
	// The default is OpenSSH known_hosts verification against
	// KnownHostsPath (~/.ssh/known_hosts when empty). Without it, anything on
	// the path can impersonate the server and collect both the credentials and
	// the backup stream.
	KnownHostsPath string `yaml:"known_hosts_path,omitempty"`

	// HostKeyFingerprint pins a single host key instead of consulting
	// known_hosts, e.g. "SHA256:n3s1Xb...". Useful for containers with no
	// known_hosts file to mount.
	HostKeyFingerprint string `yaml:"host_key_fingerprint,omitempty"`

	// InsecureSkipHostKeyVerify accepts any host key. It is the pre-0.x
	// behaviour, kept only as an explicit escape hatch, and it logs a warning
	// every time the backend is constructed.
	InsecureSkipHostKeyVerify bool `yaml:"insecure_skip_host_key_verify,omitempty"`

	// SFTP public key authentication. PrivateKeyPath is an OpenSSH private key
	// file; PrivateKeyPassphrase decrypts it when it is encrypted. Either or
	// both of key auth and Password may be configured — both are offered.
	PrivateKeyPath       string `yaml:"private_key_path,omitempty"`
	PrivateKeyPassphrase string `yaml:"private_key_passphrase,omitempty"`
}

// Notifier configuration
type Notifier struct {
	Type      string `yaml:"type"`
	URL       string `yaml:"url"`
	OnSuccess bool   `yaml:"on_success"`

	// Slack-specific configuration
	// Note: Incoming webhooks may or may not honor channel overrides depending on Slack settings.
	Channel string `yaml:"channel,omitempty"`

	// Email-specific configuration
	SMTPHost     string   `yaml:"smtp_host,omitempty"`
	SMTPPort     int      `yaml:"smtp_port,omitempty"`
	SMTPUsername string   `yaml:"smtp_username,omitempty"`
	SMTPPassword string   `yaml:"smtp_password,omitempty"`
	SMTPFrom     string   `yaml:"smtp_from,omitempty"`
	SMTPTo       []string `yaml:"smtp_to,omitempty"`
	SMTPUseTLS   bool     `yaml:"smtp_use_tls,omitempty"`

	// Webhook-specific configuration
	WebhookMethod  string            `yaml:"webhook_method,omitempty"`
	WebhookHeaders map[string]string `yaml:"webhook_headers,omitempty"`
	WebhookAuth    *WebhookAuth      `yaml:"webhook_auth,omitempty"`
}

// WebhookAuth defines webhook authentication configuration
type WebhookAuth struct {
	Type        string `yaml:"type"` // "basic", "bearer", "header"
	Username    string `yaml:"username,omitempty"`
	Password    string `yaml:"password,omitempty"`
	Token       string `yaml:"token,omitempty"`
	HeaderName  string `yaml:"header_name,omitempty"`
	HeaderValue string `yaml:"header_value,omitempty"`
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
	Name         string         `yaml:"name"`
	Conn         *Connection    `yaml:"conn"`
	SourceTarget string         `yaml:"source_target,omitempty"`
	Storage      *TargetStorage `yaml:"storage,omitempty"`
	Description  string         `yaml:"description,omitempty"`
}
