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
  config: Record<string, any>
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface StorageRequest {
  name: string
  type: 'local' | 's3' | 'sftp'
  keep: number
  config: Record<string, any>
  secret_access_key?: string
  password?: string
}

export interface Notifier {
  name: string
  type: 'slack' | 'email' | 'webhook'
  on_success: boolean
  config: Record<string, any>
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface NotifierRequest {
  name: string
  type: 'slack' | 'email' | 'webhook'
  on_success: boolean
  config: Record<string, any>
  smtp_password?: string
  webhook_auth_password?: string
  webhook_auth_token?: string
  webhook_auth_header_value?: string
}

export interface ConnectionConfig {
  type: 'mysql' | 'postgres' | 'redis'
  user?: string
  password?: string
  database?: string
  host: string
  port: number
}

export interface CompressionConfig {
  enabled: boolean
  type?: 'gzip' | 'tgz'
}

export interface TargetConfig {
  name: string
  connection: Record<string, any>
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
  connection: Record<string, any>
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
