import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AuthError, apiClient, fetchCurrentUser, login, logout, onAuthFailure } from './client'

describe('API Client', () => {
  let mockFetch: ReturnType<typeof vi.fn>

  beforeEach(() => {
    mockFetch = vi.fn()
    globalThis.fetch = mockFetch
    sessionStorage.clear()
    onAuthFailure(null)
  })

  afterEach(() => {
    vi.clearAllMocks()
    onAuthFailure(null)
  })

  describe('Authentication Functions', () => {
    it('logs in via the API and stores nothing client-side', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ username: 'testuser' }),
      })

      const user = await login('testuser', 'testpass')

      expect(user).toEqual({ username: 'testuser' })
      expect(mockFetch).toHaveBeenCalledWith('/api/login', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'testuser', password: 'testpass' }),
      })
    })

    // The whole point of the change: a password must not be recoverable from
    // the browser, so nothing credential-shaped may be written to storage.
    it('never writes credentials to web storage', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ username: 'testuser' }),
      })

      await login('testuser', 'testpass')

      expect(sessionStorage.getItem('bared_auth')).toBeNull()
      expect(sessionStorage.length).toBe(0)
      expect(localStorage.getItem('bared_auth')).toBeFalsy()
      expect(JSON.stringify(sessionStorage)).not.toContain('testpass')
      expect(JSON.stringify(localStorage)).not.toContain('testpass')
    })

    it('surfaces a failed login', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        json: async () => ({ message: 'Invalid username or password' }),
      })

      await expect(login('testuser', 'wrong')).rejects.toThrow('Invalid username or password')
    })

    it('logs out via the API', async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({}) })

      await logout()

      expect(mockFetch).toHaveBeenCalledWith('/api/logout', {
        method: 'POST',
        credentials: 'same-origin',
      })
    })

    it('does not trap the user when logout fails', async () => {
      mockFetch.mockRejectedValueOnce(new Error('network down'))

      await expect(logout()).resolves.toBeUndefined()
    })

    it('resolves the current user from the server', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ username: 'testuser' }),
      })

      await expect(fetchCurrentUser()).resolves.toEqual({ username: 'testuser' })
      expect(mockFetch).toHaveBeenCalledWith('/api/me', { credentials: 'same-origin' })
    })

    it('reports no user on 401', async () => {
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401, json: async () => ({}) })

      await expect(fetchCurrentUser()).resolves.toBeNull()
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
            credentials: 'same-origin',
            headers: expect.objectContaining({
              'Content-Type': 'application/json',
            }),
          })
        )
        expect(result).toEqual(mockDashboard)
      })
    })

    describe('getTargets', () => {
      it('fetches targets list', async () => {
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
            credentials: 'same-origin',
          })
        )
        expect(result).toEqual(mockTargets)
      })
    })

    describe('getRestoreTargets', () => {
      it('fetches restore targets list', async () => {
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
        const mockJobs = { jobs: [], total: 0 }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJobs,
        })

        await apiClient.getJobs({ status: 'running' })

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs?status=running', expect.any(Object))
      })

      it('fetches jobs with target filter', async () => {
        const mockJobs = { jobs: [], total: 0 }
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => mockJobs,
        })

        await apiClient.getJobs({ target: 'db1' })

        expect(mockFetch).toHaveBeenCalledWith('/api/jobs?target=db1', expect.any(Object))
      })

      it('fetches jobs with multiple filters', async () => {
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
      // Previously this reloaded the document, throwing away the SPA and
      // re-downloading the whole bundle. Now it raises a typed error and
      // notifies the handler the router registered.
      it('raises AuthError on 401 and notifies the auth-failure handler', async () => {
        const onFailure = vi.fn()
        onAuthFailure(onFailure)

        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 401,
          statusText: 'Unauthorized',
          json: async () => ({ message: 'Unauthorized' }),
        })

        await expect(apiClient.getDashboard()).rejects.toThrow(AuthError)

        expect(onFailure).toHaveBeenCalledTimes(1)
      })

      it('does not reload the document on 401', async () => {
        const reload = vi.fn()
        Object.defineProperty(window, 'location', {
          writable: true,
          value: { ...window.location, reload },
        })

        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 401,
          statusText: 'Unauthorized',
          json: async () => ({ message: 'Unauthorized' }),
        })

        await expect(apiClient.getDashboard()).rejects.toThrow(AuthError)

        expect(reload).not.toHaveBeenCalled()
      })

      it('handles API errors with error message from response', async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 400,
          statusText: 'Bad Request',
          json: async () => ({ message: 'Invalid target' }),
        })

        await expect(apiClient.triggerBackup('invalid')).rejects.toThrow('Invalid target')
      })

      it('handles API errors with status text fallback', async () => {
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

      it('sends the session cookie instead of an Authorization header', async () => {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => ({ targets: [] }),
        })

        await apiClient.getDashboard()

        const [, options] = mockFetch.mock.calls[0]
        expect(options.credentials).toBe('same-origin')
        expect(options.headers).not.toHaveProperty('Authorization')
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
