export interface Job {
  id: string
  type: 'backup' | 'restore'
  target: string
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelling' | 'cancelled'
  progress?: Progress
  error?: string
  created_at: string
  started_at?: string
  completed_at?: string
  duration_seconds?: number
  manual: boolean
  backup_path?: string
  target_schedule?: string // The target's current schedule (cron expression)
  triggered_by?: 'manual' | 'schedule' | 'api' // How the job was triggered
}

export interface Progress {
  stage: string
  percent: number
  bytes_processed: number
  bytes_total: number
  eta?: string
  message: string
}

export interface Target {
  name: string
  type: string
  database: string
  last_backup?: string
  next_scheduled?: string
  schedule?: string
  is_running: boolean
}

export interface LogEntry {
  timestamp: string
  level: string
  message: string
}

export interface Dashboard {
  targets: Target[]
  active_jobs: number
  total_jobs: number
  total_storage_bytes?: number
}

export interface RestoreTarget {
  name: string
  type: string
  database: string
  host: string
  description?: string
  source_target?: string
}

export interface PaginationMetadata {
  page: number
  limit: number
  offset: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

// Config Management Types

export type ConfigSource = 'database' | 'yaml'

export interface Storage {
  name: string
  type: 'local' | 's3' | 'sftp'
  keep: number
  config: Record<string, any> // eslint-disable-line @typescript-eslint/no-explicit-any -- Dynamic storage config
  enabled: boolean
  created_at: string
  updated_at: string
}

export type StorageType = 'local' | 's3' | 'sftp'

/**
 * Storage config payloads.
 *
 * These key names are the wire contract with the Go API: `requestToStorage` in
 * `apps/api/internal/api/config_handlers.go` reads exactly these keys off
 * `StorageRequest.Config` and silently drops anything else, so a typo here is a
 * setting the server never sees. Keep them in sync with `config.Storage` in
 * `apps/api/internal/config/config.go`.
 */
export interface LocalStorageConfigRequest {
  path: string
}

export interface S3StorageConfigRequest {
  bucket: string
  region: string
  access_key_id: string
  /** `endpoint_url` — not `endpoint`; blank means AWS S3. */
  endpoint_url: string
}

export interface SftpStorageConfigRequest {
  host: string
  /** Decoded as a JSON number by the API; never send a string. */
  port: number
  /** `username` — not `user`; the API ignores `user` entirely. */
  username: string
  path: string
  /** Blank falls back to `~/.ssh/known_hosts`. */
  known_hosts_path: string
  /** e.g. `SHA256:…`; mutually exclusive with insecure_skip_host_key_verify. */
  host_key_fingerprint: string
  private_key_path: string
  /** Disables MITM protection. The API rejects this alongside a fingerprint. */
  insecure_skip_host_key_verify: boolean
}

export type StorageConfigRequest =
  LocalStorageConfigRequest | S3StorageConfigRequest | SftpStorageConfigRequest

export interface StorageRequest {
  name: string
  type: StorageType
  keep: number
  config: StorageConfigRequest
  // Secrets ride outside `config` so the API never echoes them back.
  secret_access_key?: string
  password?: string
  private_key_passphrase?: string
}

// Webhook notifier config for type narrowing
export interface WebhookNotifierConfig {
  url: string
  method?: string
  headers?: Record<string, string>
  auth_type?: string
  auth_username?: string
  auth_password?: string
  auth_token?: string
  auth_header_name?: string
  auth_header_value?: string
}

export type NotifierType = 'slack' | 'email' | 'webhook'

export interface Notifier {
  name: string
  type: NotifierType
  on_success: boolean
  config: Record<string, any> // eslint-disable-line @typescript-eslint/no-explicit-any -- Dynamic notifier config
  enabled: boolean
  created_at: string
  updated_at: string
}

/**
 * Notifier config payloads.
 *
 * These key names are the wire contract with the Go API: `requestToNotifier` in
 * `apps/api/internal/api/config_handlers.go` reads exactly these keys off
 * `NotifierRequest.Config` and silently drops anything else — which is how the
 * dashboard shipped sending `webhook_url` for Slack while the API read `url`,
 * so the notifier saved and then never fired. `notifierToResponse` in the same
 * file emits the same names back, so these types describe both directions.
 * Keep them in sync with `config.Notifier` in `apps/api/internal/config/config.go`.
 */
export interface SlackNotifierConfigRequest {
  /** `url` — not `webhook_url`; the API ignores `webhook_url` entirely. */
  url: string
  channel: string
}

export interface EmailNotifierConfigRequest {
  smtp_host: string
  /** Decoded as a JSON number (`.(float64)`); never send a string. */
  smtp_port: number
  /** `smtp_username` — not `smtp_user`. */
  smtp_username: string
  /** `smtp_from` — not `from_email`. */
  smtp_from: string
  /** `smtp_to` — a list, not a single `to_email` string. */
  smtp_to: string[]
  smtp_use_tls: boolean
}

export type WebhookAuthType = 'basic' | 'bearer' | 'header'

/**
 * Nested under `webhook_auth`; the API only builds a `WebhookAuth` — and only
 * then applies the top-level auth secrets — when this object is present.
 */
export interface WebhookAuthRequest {
  type: WebhookAuthType
  username?: string
  header_name?: string
}

export interface WebhookNotifierConfigRequest {
  url: string
  /** `webhook_method` — not `method`. */
  webhook_method: string
  webhook_headers?: Record<string, string>
  webhook_auth?: WebhookAuthRequest
}

export type NotifierConfigRequest =
  SlackNotifierConfigRequest | EmailNotifierConfigRequest | WebhookNotifierConfigRequest

export interface NotifierRequest {
  name: string
  type: NotifierType
  on_success: boolean
  config: NotifierConfigRequest
  // Secrets ride outside `config` so the API never echoes them back.
  smtp_password?: string
  webhook_auth_password?: string
  webhook_auth_token?: string
  webhook_auth_header_value?: string
}

// Connection config types
export interface MySQLConnectionConfig {
  type: 'mysql'
  host: string
  port: number
  user: string
  password: string
  database: string
}

export interface PostgresConnectionConfig {
  type: 'postgres'
  host: string
  port: number
  user: string
  password: string
  database: string
}

export interface RedisConnectionConfig {
  type: 'redis'
  host: string
  port: number
  password?: string
  db?: number
}

export type ConnectionConfig =
  MySQLConnectionConfig | PostgresConnectionConfig | RedisConnectionConfig

export interface CompressionConfig {
  enabled: boolean
  type?: 'gzip' | 'tgz'
}

export interface TargetConfig {
  name: string
  connection: ConnectionConfig
  storage_name?: string
  schedule?: string
  compress?: CompressionConfig
  exclude_tables?: string[]
  additional_args?: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface TargetConfigRequest {
  name: string
  connection: ConnectionConfig
  storage_name?: string
  schedule?: string
  compress?: CompressionConfig
  exclude_tables?: string[]
  additional_args?: string[]
}

export interface RestoreTargetConfig {
  name: string
  connection: ConnectionConfig
  storage_name?: string
  source_target?: string
  description?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface RestoreTargetConfigRequest {
  name: string
  connection: ConnectionConfig
  storage_name?: string
  source_target?: string
  description?: string
}

export interface GlobalConfig {
  default_storage: string
  log_level: string
  log_format: string
  source: ConfigSource
}

export interface ConfigSourceResponse {
  source: ConfigSource
}

export interface MigrateConfigResult {
  message: string
  storages_count?: number
  notifiers_count?: number
  targets_count?: number
  restore_targets_count?: number
  global_settings_count?: number
}

export interface ReloadConfigResult {
  message: string
  source?: ConfigSource
  storages_count?: number
  notifiers_count?: number
  targets_count?: number
  restore_targets_count?: number
  schedules_updated?: boolean
}

// Config Import Types

export interface ConfigImportRequest {
  yaml_content: string
  conflict_mode: 'override' | 'skip'
  dry_run: boolean
}

export interface FailedImportResource {
  name: string
  error: string
}

export interface FailedImportConfig {
  key: string
  error: string
}

export interface ResourceImportSummary {
  created: string[]
  updated: string[]
  skipped: string[]
  failed: FailedImportResource[]
}

export interface GlobalConfigImportSummary {
  updated: string[]
  skipped: string[]
  failed: FailedImportConfig[]
}

export interface ConfigImportResponse {
  storages: ResourceImportSummary
  notifiers: ResourceImportSummary
  targets: ResourceImportSummary
  restore_targets: ResourceImportSummary
  global_config: GlobalConfigImportSummary
  dry_run: boolean
  total_created: number
  total_updated: number
  total_skipped: number
  total_failed: number
  has_errors: boolean
}
