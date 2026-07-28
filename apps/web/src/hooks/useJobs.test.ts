import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as apiClient from '../api/client'
import {
  useCancelJob,
  useJob,
  useJobLogs,
  useJobs,
  useTriggerBackup,
  useTriggerRestore,
} from './useJobs'

// Mock the API client
vi.mock('../api/client', () => ({
  apiClient: {
    getJobs: vi.fn(),
    getJob: vi.fn(),
    getJobLogs: vi.fn(),
    triggerBackup: vi.fn(),
    triggerRestore: vi.fn(),
    cancelJob: vi.fn(),
  },
}))

describe('useJobs Hooks', () => {
  let queryClient: QueryClient

  const createWrapper = () => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    return ({ children }: { children: React.ReactNode }) =>
      React.createElement(QueryClientProvider, { client: queryClient }, children)
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('useJobs', () => {
    it('fetches jobs without filters', async () => {
      const mockJobs = {
        jobs: [
          {
            id: 'job1',
            type: 'backup' as const,
            target: 'db1',
            status: 'running' as const,
            created_at: '2025-12-09T10:00:00Z',
            manual: false,
          },
          {
            id: 'job2',
            type: 'restore' as const,
            target: 'db2',
            status: 'completed' as const,
            created_at: '2025-12-09T09:00:00Z',
            manual: true,
          },
        ],
        total: 2,
      }
      vi.spyOn(apiClient.apiClient, 'getJobs').mockResolvedValueOnce(mockJobs)

      const { result } = renderHook(() => useJobs(), { wrapper: createWrapper() })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.getJobs).toHaveBeenCalledWith(undefined)
      expect(result.current.data).toEqual(mockJobs)
    })

    it('fetches jobs with status filter', async () => {
      const mockJobs = {
        jobs: [
          {
            id: 'job1',
            type: 'backup' as const,
            target: 'db1',
            status: 'running' as const,
            created_at: '2025-12-09T10:00:00Z',
            manual: false,
          },
        ],
        total: 1,
      }
      vi.spyOn(apiClient.apiClient, 'getJobs').mockResolvedValueOnce(mockJobs)

      const { result } = renderHook(() => useJobs({ status: 'running' }), {
        wrapper: createWrapper(),
      })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.getJobs).toHaveBeenCalledWith({ status: 'running' })
      expect(result.current.data).toEqual(mockJobs)
    })

    it('fetches jobs with target filter', async () => {
      const mockJobs = {
        jobs: [
          {
            id: 'job1',
            type: 'backup' as const,
            target: 'db1',
            status: 'running' as const,
            created_at: '2025-12-09T10:00:00Z',
            manual: false,
          },
        ],
        total: 1,
      }
      vi.spyOn(apiClient.apiClient, 'getJobs').mockResolvedValueOnce(mockJobs)

      const { result } = renderHook(() => useJobs({ target: 'db1' }), {
        wrapper: createWrapper(),
      })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.getJobs).toHaveBeenCalledWith({ target: 'db1' })
    })

    it('uses correct query key for caching', async () => {
      vi.spyOn(apiClient.apiClient, 'getJobs').mockResolvedValueOnce({ jobs: [], total: 0 })

      const { result } = renderHook(() => useJobs({ status: 'running' }), {
        wrapper: createWrapper(),
      })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      const cachedData = queryClient.getQueryData(['jobs', { status: 'running' }])
      expect(cachedData).toEqual({ jobs: [], total: 0 })
    })
  })

  describe('useJob', () => {
    it('fetches single job by ID', async () => {
      const mockJob = {
        id: 'job1',
        type: 'backup' as const,
        target: 'db1',
        status: 'running' as const,
        created_at: '2025-12-09T10:00:00Z',
        manual: false,
      }
      vi.spyOn(apiClient.apiClient, 'getJob').mockResolvedValueOnce(mockJob)

      const { result } = renderHook(() => useJob('job1'), { wrapper: createWrapper() })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.getJob).toHaveBeenCalledWith('job1')
      expect(result.current.data).toEqual(mockJob)
    })

    it('does not fetch when ID is empty', async () => {
      const { result } = renderHook(() => useJob(''), { wrapper: createWrapper() })

      expect(result.current.fetchStatus).toBe('idle')
      expect(apiClient.apiClient.getJob).not.toHaveBeenCalled()
    })
  })

  describe('useJobLogs', () => {
    it('fetches job logs by ID', async () => {
      const mockLogs = {
        job_id: 'job1',
        logs: [
          { timestamp: '2025-12-09T10:00:00Z', level: 'info', message: 'Starting backup' },
          { timestamp: '2025-12-09T10:01:00Z', level: 'info', message: 'Backup completed' },
        ],
        total: 2,
      }
      vi.spyOn(apiClient.apiClient, 'getJobLogs').mockResolvedValueOnce(mockLogs)

      const { result } = renderHook(() => useJobLogs('job1'), { wrapper: createWrapper() })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.getJobLogs).toHaveBeenCalledWith('job1')
      expect(result.current.data).toEqual(mockLogs)
    })

    it('does not fetch when ID is empty', async () => {
      const { result } = renderHook(() => useJobLogs(''), { wrapper: createWrapper() })

      expect(result.current.fetchStatus).toBe('idle')
      expect(apiClient.apiClient.getJobLogs).not.toHaveBeenCalled()
    })
  })

  describe('useTriggerBackup', () => {
    it('triggers backup and invalidates queries', async () => {
      const mockJob = { job_id: 'new-job', message: 'Backup started' }
      vi.spyOn(apiClient.apiClient, 'triggerBackup').mockResolvedValueOnce(mockJob)

      const { result } = renderHook(() => useTriggerBackup(), { wrapper: createWrapper() })

      result.current.mutate('db1')

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.triggerBackup).toHaveBeenCalledWith('db1')
      expect(result.current.data).toEqual(mockJob)
    })

    it('successfully completes backup trigger', async () => {
      const mockJob = { job_id: 'new-job', message: 'Backup started' }
      vi.spyOn(apiClient.apiClient, 'triggerBackup').mockResolvedValueOnce(mockJob)

      const { result } = renderHook(() => useTriggerBackup(), { wrapper: createWrapper() })

      const data = await result.current.mutateAsync('db1')

      expect(data).toEqual(mockJob)
      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })
    })

    it('handles backup trigger errors', async () => {
      vi.spyOn(apiClient.apiClient, 'triggerBackup').mockRejectedValueOnce(
        new Error('Backup failed')
      )

      const { result } = renderHook(() => useTriggerBackup(), { wrapper: createWrapper() })

      result.current.mutate('db1')

      await waitFor(() => {
        expect(result.current.isError).toBe(true)
      })

      expect(result.current.error).toBeInstanceOf(Error)
    })
  })

  describe('useTriggerRestore', () => {
    it('triggers restore with required parameters', async () => {
      const mockJob = { job_id: 'restore-job', message: 'Restore started' }
      vi.spyOn(apiClient.apiClient, 'triggerRestore').mockResolvedValueOnce(mockJob)

      const { result } = renderHook(() => useTriggerRestore(), { wrapper: createWrapper() })

      result.current.mutate({
        target: 'db1',
        backup_path: '/backups/backup-2025-12-09.sql',
      })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.triggerRestore).toHaveBeenCalledWith(
        'db1',
        '/backups/backup-2025-12-09.sql',
        undefined
      )
      expect(result.current.data).toEqual(mockJob)
    })

    it('triggers restore with dry_run option', async () => {
      const mockJob = { job_id: 'restore-job', message: 'Restore started' }
      vi.spyOn(apiClient.apiClient, 'triggerRestore').mockResolvedValueOnce(mockJob)

      const { result } = renderHook(() => useTriggerRestore(), { wrapper: createWrapper() })

      result.current.mutate({
        target: 'db1',
        backup_path: '/backups/backup-2025-12-09.sql',
        dry_run: true,
      })

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.triggerRestore).toHaveBeenCalledWith(
        'db1',
        '/backups/backup-2025-12-09.sql',
        true
      )
    })

    it('successfully completes restore trigger', async () => {
      const mockJob = { job_id: 'restore-job', message: 'Restore started' }
      vi.spyOn(apiClient.apiClient, 'triggerRestore').mockResolvedValueOnce(mockJob)

      const { result } = renderHook(() => useTriggerRestore(), { wrapper: createWrapper() })

      const data = await result.current.mutateAsync({
        target: 'db1',
        backup_path: '/backups/backup-2025-12-09.sql',
      })

      expect(data).toEqual(mockJob)
      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })
    })
  })

  describe('useCancelJob', () => {
    it('cancels job by ID', async () => {
      vi.spyOn(apiClient.apiClient, 'cancelJob').mockResolvedValueOnce({ message: 'Job cancelled' })

      const { result } = renderHook(() => useCancelJob(), { wrapper: createWrapper() })

      result.current.mutate('job1')

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })

      expect(apiClient.apiClient.cancelJob).toHaveBeenCalledWith('job1')
    })

    it('successfully completes job cancellation', async () => {
      const mockResponse = { message: 'Job cancelled' }
      vi.spyOn(apiClient.apiClient, 'cancelJob').mockResolvedValueOnce(mockResponse)

      const { result } = renderHook(() => useCancelJob(), { wrapper: createWrapper() })

      const data = await result.current.mutateAsync('job1')

      expect(data).toEqual(mockResponse)
      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true)
      })
    })

    it('handles cancel job errors', async () => {
      vi.spyOn(apiClient.apiClient, 'cancelJob').mockRejectedValueOnce(new Error('Cancel failed'))

      const { result } = renderHook(() => useCancelJob(), { wrapper: createWrapper() })

      result.current.mutate('job1')

      await waitFor(() => {
        expect(result.current.isError).toBe(true)
      })

      expect(result.current.error).toBeInstanceOf(Error)
    })

    describe('optimistic update', () => {
      const listKey = ['jobs', { status: 'running' }]
      const running = {
        id: 'job1',
        type: 'backup' as const,
        target: 'db1',
        status: 'running' as const,
        created_at: '2025-12-09T10:00:00Z',
        manual: false,
      }

      const seed = () => {
        queryClient.setQueryData(listKey, { jobs: [running], total: 1 })
        queryClient.setQueryData(['jobs', 'job1'], running)
      }

      // The daemon sets `cancelling` the moment it accepts the request, so
      // showing it straight away is the truth arriving early — not a spinner.
      it('marks the job cancelling before the request resolves', async () => {
        let release: (_value: { message: string }) => void = () => {}
        vi.spyOn(apiClient.apiClient, 'cancelJob').mockReturnValueOnce(
          new Promise((resolve) => {
            release = resolve
          })
        )

        const wrapper = createWrapper()
        seed()
        const { result } = renderHook(() => useCancelJob(), { wrapper })

        result.current.mutate('job1')

        await waitFor(() => {
          const cached = queryClient.getQueryData(listKey) as { jobs: { status: string }[] }
          expect(cached.jobs[0].status).toBe('cancelling')
        })
        expect((queryClient.getQueryData(['jobs', 'job1']) as { status: string }).status).toBe(
          'cancelling'
        )

        release({ message: 'ok' })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
      })

      it('rolls the row back when the cancel is refused', async () => {
        vi.spyOn(apiClient.apiClient, 'cancelJob').mockRejectedValueOnce(new Error('Nope'))

        const wrapper = createWrapper()
        seed()
        const { result } = renderHook(() => useCancelJob(), { wrapper })

        result.current.mutate('job1')

        await waitFor(() => expect(result.current.isError).toBe(true))

        const cached = queryClient.getQueryData(listKey) as { jobs: { status: string }[] }
        expect(cached.jobs[0].status).toBe('running')
        expect((queryClient.getQueryData(['jobs', 'job1']) as { status: string }).status).toBe(
          'running'
        )
      })
    })
  })
})
