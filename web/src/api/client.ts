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
} from '../types'

// Get auth from sessionStorage (set on login)
export const getAuthHeader = (): string => {
  const auth = sessionStorage.getItem('bared_auth')
  return auth ? `Basic ${auth}` : ''
}

// Set auth credentials
export const setAuth = (username: string, password: string) => {
  const encoded = btoa(`${username}:${password}`)
  sessionStorage.setItem('bared_auth', encoded)
}

// Clear auth
export const clearAuth = () => {
  sessionStorage.removeItem('bared_auth')
}

// Logout
export const logout = () => {
  clearAuth()
}

// Check if authenticated
export const isAuthenticated = (): boolean => {
  return !!sessionStorage.getItem('bared_auth')
}

// Base fetch with auth
const apiFetch = async (url: string, options: RequestInit = {}) => {
  const auth = getAuthHeader()

  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(auth && { Authorization: auth }),
      ...options.headers,
    },
  })

  if (response.status === 401) {
    clearAuth()
    window.location.reload()
    throw new Error('Authentication required')
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
}
