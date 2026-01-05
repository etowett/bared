import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as apiClient from '../api/client'
import { useRestoreTargets } from './useRestoreTargets'

// Mock the API client
vi.mock('../api/client', () => ({
  apiClient: {
    getRestoreTargets: vi.fn(),
  },
}))

describe('useRestoreTargets Hook', () => {
  let queryClient: QueryClient

  const createWrapper = () => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, refetchInterval: false, refetchOnWindowFocus: false },
      },
    })
    return ({ children }: { children: React.ReactNode }) =>
      React.createElement(QueryClientProvider, { client: queryClient }, children)
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    queryClient?.clear()
  })

  it('fetches restore targets data on mount', async () => {
    const mockRestoreTargets = {
      restore_targets: [
        {
          name: 'restore-db1',
          type: 'postgres',
          database: 'test_db',
          host: 'localhost',
        },
        {
          name: 'restore-db2',
          type: 'mysql',
          database: 'prod_db',
          host: 'prod-server',
          description: 'Production restore target',
          source_target: 'backup-prod',
        },
      ],
      total: 2,
    }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValueOnce(mockRestoreTargets)

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getRestoreTargets).toHaveBeenCalledTimes(1)
    expect(result.current.data).toEqual(mockRestoreTargets)
  })

  it('returns loading state initially', () => {
    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.data).toBeUndefined()
  })

  it('handles error when restore targets fetch fails', async () => {
    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockRejectedValueOnce(
      new Error('Failed to fetch restore targets')
    )

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toBeInstanceOf(Error)
  })

  it('uses correct query key for caching', async () => {
    const mockRestoreTargets = { restore_targets: [], total: 0 }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValueOnce(mockRestoreTargets)

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const cachedData = queryClient.getQueryData(['restore-targets'])
    expect(cachedData).toEqual(mockRestoreTargets)
  })

  it('caches data for 30 seconds (staleTime)', async () => {
    const mockRestoreTargets = { restore_targets: [], total: 0 }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValue(mockRestoreTargets)

    const { result: result1, unmount: unmount1 } = renderHook(() => useRestoreTargets(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getRestoreTargets).toHaveBeenCalledTimes(1)

    unmount1()

    // Mount again within staleTime (30 seconds)
    vi.advanceTimersByTime(20000) // 20 seconds

    const { result: result2 } = renderHook(() => useRestoreTargets(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true)
    })

    // Should still be called once (using cached data within staleTime)
    expect(apiClient.apiClient.getRestoreTargets).toHaveBeenCalledTimes(1)
  })

  it('refetches data after staleTime expires', async () => {
    const mockRestoreTargets = { restore_targets: [], total: 0 }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValue(mockRestoreTargets)

    const { result: result1, unmount: unmount1 } = renderHook(() => useRestoreTargets(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getRestoreTargets).toHaveBeenCalledTimes(1)

    unmount1()

    // Mount again after staleTime (30 seconds)
    vi.advanceTimersByTime(31000)

    const { result: result2 } = renderHook(() => useRestoreTargets(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true)
    })

    // Should be called twice (refetch after staleTime)
    await waitFor(() => {
      expect(apiClient.apiClient.getRestoreTargets).toHaveBeenCalledTimes(2)
    })
  })

  it('returns empty restore targets array', async () => {
    const mockRestoreTargets = { restore_targets: [], total: 0 }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValueOnce(mockRestoreTargets)

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.restore_targets).toHaveLength(0)
    expect(result.current.data?.total).toBe(0)
  })

  it('returns restore targets with complete information', async () => {
    const mockRestoreTargets = {
      restore_targets: [
        {
          name: 'restore-prod',
          type: 'postgres',
          database: 'production',
          host: 'prod-restore.example.com',
          description: 'Production database restore point',
          source_target: 'prod-backup',
        },
      ],
      total: 1,
    }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValueOnce(mockRestoreTargets)

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.restore_targets[0]).toEqual({
      name: 'restore-prod',
      type: 'postgres',
      database: 'production',
      host: 'prod-restore.example.com',
      description: 'Production database restore point',
      source_target: 'prod-backup',
    })
  })

  it('returns restore targets with optional fields', async () => {
    const mockRestoreTargets = {
      restore_targets: [
        {
          name: 'restore-db',
          type: 'mysql',
          database: 'app_db',
          host: 'localhost',
          // Optional fields not provided
        },
      ],
      total: 1,
    }

    vi.spyOn(apiClient.apiClient, 'getRestoreTargets').mockResolvedValueOnce(mockRestoreTargets)

    const { result } = renderHook(() => useRestoreTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const target = result.current.data?.restore_targets[0]
    expect(target?.name).toBe('restore-db')
    expect(target?.description).toBeUndefined()
    expect(target?.source_target).toBeUndefined()
  })
})
