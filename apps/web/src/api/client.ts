import type {
  Dashboard,
  Job,
  LogEntry,
  PaginationMetadata,
  RestoreTarget,
  Target,
  Storage,
  StorageRequest,
  Notifier,
  NotifierRequest,
  TargetConfig,
  TargetConfigRequest,
  RestoreTargetConfig,
  RestoreTargetConfigRequest,
  GlobalConfig,
  ConfigSourceResponse,
  ConfigImportRequest,
  ConfigImportResponse,
} from '../types'

/**
 * Thrown when the server rejects a request as unauthenticated.
 *
 * The client deliberately does not navigate on its own — routing from here
 * would mean reaching for `window.location` (a full page reload) or importing
 * the router (a circular import). Callers subscribe via `onAuthFailure`.
 */
export class AuthError extends Error {
  constructor(message = 'Authentication required') {
    super(message)
    this.name = 'AuthError'
  }
}

type AuthFailureHandler = () => void

let authFailureHandler: AuthFailureHandler | null = null

/** Registers the callback invoked whenever a request comes back 401. */
export const onAuthFailure = (handler: AuthFailureHandler | null) => {
  authFailureHandler = handler
}

/**
 * Logs in and lets the server set an httpOnly session cookie.
 *
 * Credentials are never stored client-side: the browser holds an httpOnly
 * cookie that JavaScript cannot read, so an XSS payload has nothing to
 * exfiltrate. It also rides along on the WebSocket handshake, which is the one
 * request where the browser cannot set an Authorization header.
 */
export const login = async (username: string, password: string): Promise<{ username: string }> => {
  const response = await fetch('/api/login', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: response.statusText }))
    throw new Error(error.message || 'Invalid username or password')
  }

  return response.json()
}

/** Ends the session server-side, which also closes its live log streams. */
export const logout = async (): Promise<void> => {
  await fetch('/api/logout', {
    method: 'POST',
    credentials: 'same-origin',
  }).catch(() => {
    // A failed logout must not trap the user in the app — the client-side
    // session is discarded either way.
  })
}

/**
 * Resolves the current identity, or null when unauthenticated.
 *
 * This replaces reading a token out of storage: the session cookie is httpOnly,
 * so only the server can answer whether it is still valid.
 */
export const fetchCurrentUser = async (): Promise<{ username: string } | null> => {
  const response = await fetch('/api/me', { credentials: 'same-origin' })

  if (response.status === 401) {
    return null
  }
  if (!response.ok) {
    throw new Error('Failed to check authentication')
  }

  return response.json()
}

// Base fetch with auth
const apiFetch = async (url: string, options: RequestInit = {}) => {
  const response = await fetch(url, {
    ...options,
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  })

  if (response.status === 401) {
    authFailureHandler?.()
    throw new AuthError()
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: response.statusText }))
    throw new Error(error.message || 'Request failed')
  }

  return response.json()
}

// API methods
export const apiClient = {
  // Health check
  async health(): Promise<{ status: string; version: string }> {
    return fetch('/api/health').then((r) => r.json())
  },

  // Dashboard
  async getDashboard(): Promise<Dashboard> {
    return apiFetch('/api/dashboard')
  },

  // Targets
  async getTargets(): Promise<{ targets: Target[]; total: number }> {
    return apiFetch('/api/targets')
  },

  // Restore Targets
  async getRestoreTargets(): Promise<{ restore_targets: RestoreTarget[]; total: number }> {
    return apiFetch('/api/restore-targets')
  },

  // Jobs
  async getJobs(filters?: {
    status?: string
    target?: string
    type?: 'backup' | 'restore'
    page?: number
    limit?: number
  }): Promise<{ jobs: Job[]; total: number; pagination: PaginationMetadata }> {
    const params = new URLSearchParams()
    if (filters?.status) params.set('status', filters.status)
    if (filters?.target) params.set('target', filters.target)
    if (filters?.type) params.set('type', filters.type)
    if (filters?.page !== undefined) params.set('page', filters.page.toString())
    if (filters?.limit !== undefined) params.set('limit', filters.limit.toString())

    const url = `/api/jobs${params.toString() ? `?${params}` : ''}`
    return apiFetch(url)
  },

  async getJob(id: string): Promise<Job> {
    return apiFetch(`/api/jobs/${id}`)
  },

  async triggerBackup(target: string): Promise<{ job_id: string; message: string }> {
    return apiFetch('/api/jobs/backup', {
      method: 'POST',
      body: JSON.stringify({ target }),
    })
  },

  async triggerRestore(
    target: string,
    backup_path: string,
    dry_run?: boolean
  ): Promise<{ job_id: string; message: string }> {
    return apiFetch('/api/jobs/restore', {
      method: 'POST',
      body: JSON.stringify({ target, backup_path, dry_run: dry_run || false }),
    })
  },

  async cancelJob(id: string): Promise<{ message: string }> {
    return apiFetch(`/api/jobs/${id}`, {
      method: 'DELETE',
    })
  },

  async getJobLogs(id: string): Promise<{ job_id: string; logs: LogEntry[]; total: number }> {
    return apiFetch(`/api/jobs/${id}/logs`)
  },

  // Config Management - Storages
  async getStorages(): Promise<{ storages: Storage[]; total: number; source: string }> {
    return apiFetch('/api/config/storages')
  },

  async getStorage(name: string): Promise<Storage> {
    return apiFetch(`/api/config/storages/${name}`)
  },

  async createStorage(storage: StorageRequest): Promise<{ message: string; name: string }> {
    return apiFetch('/api/config/storages', {
      method: 'POST',
      body: JSON.stringify(storage),
    })
  },

  async updateStorage(
    name: string,
    storage: StorageRequest
  ): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/storages/${name}`, {
      method: 'PUT',
      body: JSON.stringify(storage),
    })
  },

  async deleteStorage(name: string): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/storages/${name}`, {
      method: 'DELETE',
    })
  },

  // Config Management - Notifiers
  async getNotifiers(): Promise<{ notifiers: Notifier[]; total: number; source: string }> {
    return apiFetch('/api/config/notifiers')
  },

  async createNotifier(notifier: NotifierRequest): Promise<{ message: string; name: string }> {
    return apiFetch('/api/config/notifiers', {
      method: 'POST',
      body: JSON.stringify(notifier),
    })
  },

  async updateNotifier(
    name: string,
    notifier: NotifierRequest
  ): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/notifiers/${name}`, {
      method: 'PUT',
      body: JSON.stringify(notifier),
    })
  },

  async deleteNotifier(name: string): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/notifiers/${name}`, {
      method: 'DELETE',
    })
  },

  // Config Management - Targets
  async getTargetsConfig(): Promise<{ targets: TargetConfig[]; total: number; source: string }> {
    return apiFetch('/api/config/targets')
  },

  async createTargetConfig(
    target: TargetConfigRequest
  ): Promise<{ message: string; name: string }> {
    return apiFetch('/api/config/targets', {
      method: 'POST',
      body: JSON.stringify(target),
    })
  },

  async updateTargetConfig(
    name: string,
    target: TargetConfigRequest
  ): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/targets/${name}`, {
      method: 'PUT',
      body: JSON.stringify(target),
    })
  },

  async updateTargetSchedule(
    name: string,
    schedule: string
  ): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/targets/${name}/schedule`, {
      method: 'PATCH',
      body: JSON.stringify({ schedule }),
    })
  },

  async deleteTargetConfig(name: string): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/targets/${name}`, {
      method: 'DELETE',
    })
  },

  // Config Management - Restore Targets
  async getRestoreTargetsConfig(): Promise<{
    restore_targets: RestoreTargetConfig[]
    total: number
    source: string
  }> {
    return apiFetch('/api/config/restore-targets')
  },

  async createRestoreTargetConfig(
    target: RestoreTargetConfigRequest
  ): Promise<{ message: string; name: string }> {
    return apiFetch('/api/config/restore-targets', {
      method: 'POST',
      body: JSON.stringify(target),
    })
  },

  async updateRestoreTargetConfig(
    name: string,
    target: RestoreTargetConfigRequest
  ): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/restore-targets/${name}`, {
      method: 'PUT',
      body: JSON.stringify(target),
    })
  },

  async deleteRestoreTargetConfig(name: string): Promise<{ message: string; name: string }> {
    return apiFetch(`/api/config/restore-targets/${name}`, {
      method: 'DELETE',
    })
  },

  // Config Management - Global Config
  async getGlobalConfig(): Promise<GlobalConfig> {
    return apiFetch('/api/config/global')
  },

  async updateGlobalConfig(
    key: string,
    value: string
  ): Promise<{ message: string; key: string; value: string }> {
    return apiFetch(`/api/config/global/${key}`, {
      method: 'PUT',
      body: JSON.stringify({ value }),
    })
  },

  // Config Management - Utilities
  async getConfigSource(): Promise<ConfigSourceResponse> {
    return apiFetch('/api/config/source')
  },

  async migrateConfig(): Promise<{
    message: string
    storages_count: number
    notifiers_count: number
    targets_count: number
  }> {
    return apiFetch('/api/config/migrate', {
      method: 'POST',
    })
  },

  async reloadConfig(): Promise<{ message: string; source: string }> {
    return apiFetch('/api/config/reload', {
      method: 'POST',
    })
  },

  // Config Management - Import
  async importConfig(request: ConfigImportRequest): Promise<ConfigImportResponse> {
    return apiFetch('/api/config/import', {
      method: 'POST',
      body: JSON.stringify(request),
    })
  },
}
