import type { Job, Target, Dashboard, LogEntry, RestoreTarget } from '../types'

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
  }): Promise<{ jobs: Job[]; total: number }> {
    const params = new URLSearchParams()
    if (filters?.status) params.set('status', filters.status)
    if (filters?.target) params.set('target', filters.target)

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
}
