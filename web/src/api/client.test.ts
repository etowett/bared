import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { getAuthHeader, setAuth, clearAuth, isAuthenticated, apiClient } from './client'

describe('API Client', () => {
  let mockFetch: ReturnType<typeof vi.fn>

  beforeEach(() => {
    mockFetch = vi.fn()
    globalThis.fetch = mockFetch
    sessionStorage.clear()
    // Mock window.location.reload
    Object.defineProperty(window, 'location', {
      writable: true,
      value: { reload: vi.fn() },
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  describe('Authentication Functions', () => {
    it('sets auth credentials correctly', () => {
      setAuth('testuser', 'testpass')

      const stored = sessionStorage.getItem('bared_auth')
      expect(stored).toBe(btoa('testuser:testpass'))
    })

    it('gets auth header when credentials are set', () => {
      setAuth('testuser', 'testpass')

      const header = getAuthHeader()
      expect(header).toBe(`Basic ${btoa('testuser:testpass')}`)
    })

    it('returns empty string when no auth is set', () => {
      const header = getAuthHeader()
      expect(header).toBe('')
    })

    it('clears auth credentials', () => {
      setAuth('testuser', 'testpass')
      expect(sessionStorage.getItem('bared_auth')).toBeTruthy()

      clearAuth()
      expect(sessionStorage.getItem('bared_auth')).toBeNull()
    })

    it('checks authentication status correctly', () => {
      expect(isAuthenticated()).toBe(false)

      setAuth('testuser', 'testpass')
      expect(isAuthenticated()).toBe(true)

      clearAuth()
      expect(isAuthenticated()).toBe(false)
    })
  })

  describe('API Methods', () => {
    describe('health', () => {
      it('fetches health status without auth', async () => {
        const mockResponse = { status: 'ok', version: '1.0.0' }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockResponse,
        })

        const result = await apiClient.health()

        expect(mockFetch).toHaveBeenCalledWith('/api/health')
        expect(result).toEqual(mockResponse)
      })
    })

    describe('getDashboard', () => {
      it('fetches dashboard data with auth', async () => {
        setAuth('user', 'pass')
        const mockDashboard = {
          targets: [],
          active_jobs: 0,
          total_jobs: 10,
        }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockDashboard,
        })

        const result = await apiClient.getDashboard()

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/dashboard',
          expect.objectContaining({
            headers: expect.objectContaining({
              Authorization: `Basic ${btoa('user:pass')}`,
              'Content-Type': 'application/json',
            }),
          })
        )
        expect(result).toEqual(mockDashboard)
      })
    })

    describe('getTargets', () => {
      it('fetches targets list', async () => {
        setAuth('user', 'pass')
        const mockTargets = {
          targets: [{ name: 'db1', type: 'mysql', database: 'test', is_running: false }],
          total: 1,
        }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockTargets,
        })

        const result = await apiClient.getTargets()

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/targets',
          expect.objectContaining({
            headers: expect.objectContaining({
              Authorization: expect.any(String),
            }),
          })
        )
        expect(result).toEqual(mockTargets)
      })
    })

    describe('getRestoreTargets', () => {
      it('fetches restore targets list', async () => {
        setAuth('user', 'pass')
        const mockRestoreTargets = {
          restore_targets: [
            {
              name: 'restore-db1',
              type: 'mysql',
              database: 'test',
              host: 'localhost',
            },
          ],
          total: 1,
        }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockRestoreTargets,
        })

        const result = await apiClient.getRestoreTargets()

        expect(mockFetch).toHaveBeenCalledWith('/api/restore-targets', expect.any(Object))
        expect(result).toEqual(mockRestoreTargets)
      })
    })

    describe('getJobs', () => {
      it('fetches jobs without filters', async () => {
        setAuth('user', 'pass')
        const mockJobs = {
          jobs: [{ id: 'job1', type: 'backup', target: 'db1', status: 'running' }],
          total: 1,
        }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJobs,
        })

        const result = await apiClient.getJobs()

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs', expect.any(Object))
        expect(result).toEqual(mockJobs)
      })

      it('fetches jobs with status filter', async () => {
        setAuth('user', 'pass')
        const mockJobs = { jobs: [], total: 0 }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJobs,
        })

        await apiClient.getJobs({ status: 'running' })

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs?status=running', expect.any(Object))
      })

      it('fetches jobs with target filter', async () => {
        setAuth('user', 'pass')
        const mockJobs = { jobs: [], total: 0 }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJobs,
        })

        await apiClient.getJobs({ target: 'db1' })

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs?target=db1', expect.any(Object))
      })

      it('fetches jobs with multiple filters', async () => {
        setAuth('user', 'pass')
        const mockJobs = { jobs: [], total: 0 }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJobs,
        })

        await apiClient.getJobs({ status: 'running', target: 'db1' })

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/jobs?status=running&target=db1',
          expect.any(Object)
        )
      })
    })

    describe('getJob', () => {
      it('fetches single job by ID', async () => {
        setAuth('user', 'pass')
        const mockJob = {
          id: 'job1',
          type: 'backup',
          target: 'db1',
          status: 'completed',
        }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJob,
        })

        const result = await apiClient.getJob('job1')

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs/job1', expect.any(Object))
        expect(result).toEqual(mockJob)
      })
    })

    describe('triggerBackup', () => {
      it('triggers backup for target', async () => {
        setAuth('user', 'pass')
        const mockResponse = { job_id: 'new-job', message: 'Backup started' }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockResponse,
        })

        const result = await apiClient.triggerBackup('db1')

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/jobs/backup',
          expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ target: 'db1' }),
            headers: expect.objectContaining({
              'Content-Type': 'application/json',
            }),
          })
        )
        expect(result).toEqual(mockResponse)
      })
    })

    describe('triggerRestore', () => {
      it('triggers restore with required parameters', async () => {
        setAuth('user', 'pass')
        const mockResponse = { job_id: 'restore-job', message: 'Restore started' }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockResponse,
        })

        const result = await apiClient.triggerRestore('db1', '/backups/backup.sql')

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/jobs/restore',
          expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({
              target: 'db1',
              backup_path: '/backups/backup.sql',
              dry_run: false,
            }),
          })
        )
        expect(result).toEqual(mockResponse)
      })

      it('triggers restore with dry_run enabled', async () => {
        setAuth('user', 'pass')
        const mockResponse = { job_id: 'restore-job', message: 'Restore started' }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockResponse,
        })

        await apiClient.triggerRestore('db1', '/backups/backup.sql', true)

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/jobs/restore',
          expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({
              target: 'db1',
              backup_path: '/backups/backup.sql',
              dry_run: true,
            }),
          })
        )
      })
    })

    describe('cancelJob', () => {
      it('cancels job by ID', async () => {
        setAuth('user', 'pass')
        const mockResponse = { message: 'Job cancelled' }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockResponse,
        })

        const result = await apiClient.cancelJob('job1')

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/jobs/job1',
          expect.objectContaining({
            method: 'DELETE',
          })
        )
        expect(result).toEqual(mockResponse)
      })
    })

    describe('getJobLogs', () => {
      it('fetches job logs by ID', async () => {
        setAuth('user', 'pass')
        const mockLogs = {
          job_id: 'job1',
          logs: [{ timestamp: '2025-12-09T10:00:00Z', level: 'info', message: 'Starting' }],
          total: 1,
        }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockLogs,
        })

        const result = await apiClient.getJobLogs('job1')

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs/job1/logs', expect.any(Object))
        expect(result).toEqual(mockLogs)
      })
    })

    describe('Error Handling', () => {
      it('handles 401 unauthorized by clearing auth and reloading', async () => {
        setAuth('user', 'pass')
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 401,
          statusText: 'Unauthorized',
          json: async () => ({ message: 'Unauthorized' }),
        })

        await expect(apiClient.getDashboard()).rejects.toThrow('Authentication required')

        expect(sessionStorage.getItem('bared_auth')).toBeNull()
        expect(window.location.reload).toHaveBeenCalled()
      })

      it('handles API errors with error message from response', async () => {
        setAuth('user', 'pass')
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 400,
          statusText: 'Bad Request',
          json: async () => ({ message: 'Invalid target' }),
        })

        await expect(apiClient.triggerBackup('invalid')).rejects.toThrow('Invalid target')
      })

      it('handles API errors with status text fallback', async () => {
        setAuth('user', 'pass')
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 500,
          statusText: 'Internal Server Error',
          json: async () => {
            throw new Error('Invalid JSON')
          },
        })

        await expect(apiClient.getDashboard()).rejects.toThrow('Internal Server Error')
      })

      it('includes auth header when credentials are set', async () => {
        setAuth('testuser', 'testpass')
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => ({ targets: [] }),
        })

        await apiClient.getDashboard()

        expect(mockFetch).toHaveBeenCalledWith(
          '/api/dashboard',
          expect.objectContaining({
            headers: expect.objectContaining({
              Authorization: `Basic ${btoa('testuser:testpass')}`,
            }),
          })
        )
      })

      it('works without auth header when not authenticated', async () => {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => ({ status: 'ok' }),
        })

        await apiClient.health()

        expect(mockFetch).toHaveBeenCalledWith('/api/health')
      })
    })
  })
})
