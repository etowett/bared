import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ConfirmProvider } from '@/contexts/ConfirmContext'
import { ThemeProvider } from '@/contexts/ThemeContext'
import { TooltipProvider } from '@/components/ui/tooltip'
import { routeTree } from '@/routeTree.gen'
import { useAuthStore } from '@/stores/auth'
import type { Job } from '@/types'

/**
 * These are integration tests: each one mounts the real route tree, runs the
 * router's loaders and renders the whole app shell. That is the point — they
 * assert that URL and table state round-trip, which a shallow render cannot
 * show — but it costs far more than a unit test's default 5s allows on a
 * two-core CI runner.
 *
 * It compounds, too. `useJobs` sets `refetchInterval: 3000`, so any test that
 * runs longer than three seconds triggers a refetch and a re-render, which
 * makes it slower, which invites the next refetch.
 *
 * Measured on an unloaded 12-core machine the whole file takes ~4s; under
 * enough CPU contention to model a shared runner, single tests reach 6s. So the
 * ceiling is raised here, for this file only, rather than globally — a global
 * bump would slow the failure feedback of every genuinely-hung unit test.
 */
vi.setConfig({ testTimeout: 30_000 })

const job = (overrides: Partial<Job> = {}): Job => ({
  id: '12345678-1234-1234-1234-123456789012',
  type: 'backup',
  target: 'zeta',
  status: 'completed',
  created_at: '2026-01-01T10:00:00Z',
  duration_seconds: 10,
  manual: false,
  ...overrides,
})

const getJobs = vi.fn()

vi.mock('@/api/client', () => ({
  AuthError: class AuthError extends Error {},
  onAuthFailure: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  fetchCurrentUser: vi.fn().mockResolvedValue({ username: 'admin' }),
  apiClient: {
    health: vi.fn().mockResolvedValue({ status: 'ok', version: 'test' }),
    getJobs: (...args: unknown[]) => getJobs(...args),
    getTargets: vi
      .fn()
      .mockResolvedValue({ targets: [{ name: 'zeta' }, { name: 'alpha' }], total: 2 }),
    getRestoreTargets: vi.fn().mockResolvedValue({ restore_targets: [], total: 0 }),
    getDashboard: vi.fn().mockResolvedValue({ targets: [], stats: {} }),
  },
}))

/**
 * Mounts the real route tree at `path`.
 *
 * Starting a fresh memory history at a URL is exactly what a reload does, so
 * anything this asserts is also an assertion that the state survives one.
 */
async function renderAt(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  await router.load()

  render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <TooltipProvider delayDuration={0}>
          <ConfirmProvider>
            <RouterProvider router={router} />
          </ConfirmProvider>
        </TooltipProvider>
      </QueryClientProvider>
    </ThemeProvider>
  )

  return router
}

describe('job table state in the URL', () => {
  beforeEach(() => {
    useAuthStore.setState({ status: 'authenticated', username: 'admin' })
    getJobs.mockReset()
    getJobs.mockResolvedValue({
      jobs: [
        job({ id: 'a', target: 'zeta', duration_seconds: 10 }),
        job({ id: 'b', target: 'alpha', duration_seconds: 90 }),
      ],
      total: 2,
      pagination: {
        page: 1,
        limit: 20,
        offset: 0,
        total_pages: 1,
        has_next: false,
        has_prev: false,
      },
    })
  })

  it('drives the server-side filters straight off the URL', async () => {
    await renderAt('/jobs?page=2&limit=50&status=failed&type=backup&target=zeta')

    await waitFor(() =>
      expect(getJobs).toHaveBeenCalledWith({
        page: 2,
        limit: 50,
        status: 'failed',
        type: 'backup',
        target: 'zeta',
      })
    )
  })

  it('shows the filter controls at the values the URL asked for', async () => {
    await renderAt('/jobs?status=failed&target=zeta')

    expect(await screen.findByRole('combobox', { name: /^status$/i })).toHaveTextContent('Failed')
    expect(screen.getByRole('combobox', { name: /^target$/i })).toHaveTextContent('zeta')
  })

  it('drops values the API would not understand rather than forwarding them', async () => {
    // A hand-edited status is answered by the daemon with an empty list and no
    // error, which reads as "no jobs" instead of "bad link".
    await renderAt('/jobs?status=bogus&limit=999&page=-3')

    await waitFor(() => expect(getJobs).toHaveBeenCalledWith({ page: 1, limit: 20 }))
  })

  it('restores the sort from the URL', async () => {
    await renderAt('/jobs?sort=duration&order=asc')

    const header = await screen.findByRole('columnheader', { name: /duration/i })
    expect(header).toHaveAttribute('aria-sort', 'ascending')
  })

  it('writes a changed sort back to the URL', async () => {
    const router = await renderAt('/jobs')

    fireEvent.click(await screen.findByRole('button', { name: /duration/i }))

    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ sort: 'duration', order: 'desc' })
    )
  })

  it('keeps the backup history pinned to backup jobs whatever the URL says', async () => {
    await renderAt('/backup/jobs?type=restore')

    await waitFor(() =>
      expect(getJobs).toHaveBeenCalledWith(expect.objectContaining({ type: 'backup' }))
    )
  })

  // Slowest test in the file: a Select interaction on top of a full app mount.
  // Kept last so its cost lands at the end of the run rather than in the middle.
  it('writes a changed filter back to the URL so the view stays linkable', async () => {
    const user = userEvent.setup()
    const router = await renderAt('/jobs')

    await user.click(await screen.findByRole('combobox', { name: /^status$/i }))
    await user.click(await screen.findByRole('option', { name: 'Failed' }))

    await waitFor(() => expect(router.state.location.search).toMatchObject({ status: 'failed' }))
  })
})
