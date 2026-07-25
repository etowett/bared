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

export interface StorageRequest {
  name: string
  type: 'local' | 's3' | 'sftp'
  keep: number
  config: Record<string, any> // eslint-disable-line @typescript-eslint/no-explicit-any -- Dynamic storage config
  secret_access_key?: string
  password?: string
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

export interface Notifier {
  name: string
  type: 'slack' | 'email' | 'webhook'
  on_success: boolean
  config: Record<string, any> // eslint-disable-line @typescript-eslint/no-explicit-any -- Dynamic notifier config
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface NotifierRequest {
  name: string
  type: 'slack' | 'email' | 'webhook'
  on_success: boolean
  config: Record<string, any> // eslint-disable-line @typescript-eslint/no-explicit-any -- Dynamic notifier config
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
  | MySQLConnectionConfig
  | PostgresConnectionConfig
  | RedisConnectionConfig

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
