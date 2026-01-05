import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as apiClient from '../api/client'
import { useTargets } from './useTargets'

// Mock the API client
vi.mock('../api/client', () => ({
  apiClient: {
    getTargets: vi.fn(),
  },
}))

describe('useTargets Hook', () => {
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
  })

  afterEach(() => {
    queryClient?.clear()
  })

  it('fetches targets data on mount', async () => {
    const mockTargets = {
      targets: [
        { name: 'db1', type: 'postgres', database: 'app_db', is_running: false },
        { name: 'db2', type: 'mysql', database: 'user_db', is_running: true },
      ],
      total: 2,
    }

    vi.spyOn(apiClient.apiClient, 'getTargets').mockResolvedValueOnce(mockTargets)

    const { result } = renderHook(() => useTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(apiClient.apiClient.getTargets).toHaveBeenCalledTimes(1)
    expect(result.current.data).toEqual(mockTargets)
  })

  it('returns loading state initially', () => {
    vi.spyOn(apiClient.apiClient, 'getTargets').mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    const { result } = renderHook(() => useTargets(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.data).toBeUndefined()
  })

  it('handles error when targets fetch fails', async () => {
    vi.spyOn(apiClient.apiClient, 'getTargets').mockRejectedValueOnce(
      new Error('Failed to fetch targets')
    )

    const { result } = renderHook(() => useTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toBeInstanceOf(Error)
  })

  it('uses correct query key for caching', async () => {
    const mockTargets = { targets: [], total: 0 }

    vi.spyOn(apiClient.apiClient, 'getTargets').mockResolvedValueOnce(mockTargets)

    const { result } = renderHook(() => useTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const cachedData = queryClient.getQueryData(['targets'])
    expect(cachedData).toEqual(mockTargets)
  })

  it('returns empty targets array', async () => {
    const mockTargets = { targets: [], total: 0 }

    vi.spyOn(apiClient.apiClient, 'getTargets').mockResolvedValueOnce(mockTargets)

    const { result } = renderHook(() => useTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.targets).toHaveLength(0)
    expect(result.current.data?.total).toBe(0)
  })

  it('returns targets with complete information', async () => {
    const mockTargets = {
      targets: [
        {
          name: 'prod-db',
          type: 'postgres',
          database: 'production',
          is_running: true,
          last_backup: '2025-12-09T10:00:00Z',
          schedule: '0 2 * * *',
          next_scheduled: '2025-12-10T02:00:00Z',
        },
      ],
      total: 1,
    }

    vi.spyOn(apiClient.apiClient, 'getTargets').mockResolvedValueOnce(mockTargets)

    const { result } = renderHook(() => useTargets(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.targets[0]).toEqual({
      name: 'prod-db',
      type: 'postgres',
      database: 'production',
      is_running: true,
      last_backup: '2025-12-09T10:00:00Z',
      schedule: '0 2 * * *',
      next_scheduled: '2025-12-10T02:00:00Z',
    })
  })
})
