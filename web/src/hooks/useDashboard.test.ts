import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as apiClient from '../api/client'
import { useDashboard } from './useDashboard'

// Mock the API client
vi.mock('../api/client', () => ({
  apiClient: {
    getDashboard: vi.fn(),
  },
}))

describe('useDashboard Hook', () => {
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

  it('fetches dashboard data on mount', async () => {
    const mockDashboard = {
      targets: [
        { name: 'db1', type: 'postgres', database: 'app_db', is_running: false },
      ],
      active_jobs: 2,
      total_jobs: 50,
      total_storage_bytes: 1073741824,
    }

    vi.spyOn(apiClient.apiClient, 'getDashboard').mockResolvedValueOnce(mockDashboard)

    const { result } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getDashboard).toHaveBeenCalledTimes(1)
    expect(result.current.data).toEqual(mockDashboard)
  })

  it('returns loading state initially', () => {
    vi.spyOn(apiClient.apiClient, 'getDashboard').mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    const { result } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.data).toBeUndefined()
  })

  it('handles error when dashboard fetch fails', async () => {
    vi.spyOn(apiClient.apiClient, 'getDashboard').mockRejectedValueOnce(
      new Error('Network error')
    )

    const { result } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toBeInstanceOf(Error)
  })

  it('uses correct query key for caching', async () => {
    const mockDashboard = {
      targets: [],
      active_jobs: 0,
      total_jobs: 0,
      total_storage_bytes: 0,
    }

    vi.spyOn(apiClient.apiClient, 'getDashboard').mockResolvedValueOnce(mockDashboard)

    const { result } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const cachedData = queryClient.getQueryData(['dashboard'])
    expect(cachedData).toEqual(mockDashboard)
  })

  it('auto-refreshes data every 5 seconds', async () => {
    const mockDashboard = { targets: [], active_jobs: 0, total_jobs: 0, total_storage_bytes: 0 }

    vi.spyOn(apiClient.apiClient, 'getDashboard').mockResolvedValue(mockDashboard)

    const { result } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getDashboard).toHaveBeenCalledTimes(1)

    // Fast-forward 5 seconds
    vi.advanceTimersByTime(5000)

    await waitFor(() => {
      expect(apiClient.apiClient.getDashboard).toHaveBeenCalledTimes(2)
    })
  })

  it('reuses cached data within staleTime', async () => {
    const mockDashboard = { targets: [], active_jobs: 0, total_jobs: 0, total_storage_bytes: 0 }

    vi.spyOn(apiClient.apiClient, 'getDashboard').mockResolvedValue(mockDashboard)

    const { result: result1, unmount: unmount1 } = renderHook(() => useDashboard(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getDashboard).toHaveBeenCalledTimes(1)

    unmount1()

    // Mount again within staleTime (2 seconds)
    vi.advanceTimersByTime(1000)

    const { result: result2 } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true)
    })

    // Should still be called once (using cached data)
    expect(apiClient.apiClient.getDashboard).toHaveBeenCalledTimes(1)
  })

  it('returns dashboard with targets array', async () => {
    const mockDashboard = {
      targets: [
        { name: 'db1', type: 'postgres', database: 'db1', is_running: false },
        { name: 'db2', type: 'mysql', database: 'db2', is_running: true },
      ],
      active_jobs: 1,
      total_jobs: 25,
      total_storage_bytes: 524288000,
    }

    vi.spyOn(apiClient.apiClient, 'getDashboard').mockResolvedValueOnce(mockDashboard)

    const { result } = renderHook(() => useDashboard(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.targets).toHaveLength(2)
    expect(result.current.data?.active_jobs).toBe(1)
    expect(result.current.data?.total_jobs).toBe(25)
  })
})
