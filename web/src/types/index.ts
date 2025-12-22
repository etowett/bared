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
