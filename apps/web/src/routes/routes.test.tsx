import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ThemeProvider } from '@/contexts/ThemeContext'
import { useAuthStore } from '@/stores/auth'
import { routeTree } from '@/routeTree.gen'

// The route modules reach the network through `apiClient`; the root route's
// guard goes through `fetchCurrentUser`. Both are stubbed so these tests are
// about routing only.
vi.mock('@/api/client', () => ({
  AuthError: class AuthError extends Error {},
  onAuthFailure: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  fetchCurrentUser: vi.fn().mockResolvedValue({ username: 'admin' }),
  apiClient: {
    getJobs: vi.fn().mockResolvedValue({
      jobs: [],
      total: 0,
      pagination: { page: 1, limit: 20, total: 0, total_pages: 0 },
    }),
    getTargets: vi.fn().mockResolvedValue({ targets: [], total: 0 }),
    getRestoreTargets: vi.fn().mockResolvedValue({ restore_targets: [], total: 0 }),
    getDashboard: vi.fn().mockResolvedValue({ targets: [], stats: {} }),
  },
}))

async function renderRouteAt(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [path] }),
  })

  // Resolve the match — including fetching the matched route's lazy chunk —
  // before asserting, so a missing chunk fails loudly instead of timing out.
  await router.load()

  render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>
  )

  return router
}

/**
 * Each page's own `<h2>` is its identity. Card titles are plain `<div>`s, so
 * `role="heading"` matches exactly one element per page — which is what makes
 * "the parent rendered instead of the child" detectable at all. Asserting on
 * text alone would not: `/backup` also contains the string "Backup Job
 * History", inside a card title.
 */
async function pageHeading() {
  return (await screen.findByRole('heading', { level: 2 })).textContent
}

describe('route tree rendering', () => {
  beforeEach(() => {
    useAuthStore.setState({ status: 'authenticated', username: 'admin' })
  })

  describe('nested routes render their own content, not the parent', () => {
    // Regression test for #103. `routes/backup.lazy.tsx` was a flat route that
    // made `routes/backup/jobs.lazy.tsx` its child, but rendered no `<Outlet />`
    // — so `/backup/jobs` matched, fetched its chunk, and then displayed the
    // backup page. No error, no console warning. Type-checking cannot see this,
    // and neither can a component test that renders `BackupJobsPage` directly:
    // it only shows up when the real route tree resolves the URL.
    it('/backup/jobs renders the backup job history page', async () => {
      await renderRouteAt('/backup/jobs')

      expect(await pageHeading()).toBe('Backup Job History')
      // Only the child has this link back to the parent.
      expect(screen.getByRole('link', { name: /back to targets/i })).toBeInTheDocument()
    })

    it('/restore/jobs renders the restore job history page', async () => {
      await renderRouteAt('/restore/jobs')

      expect(await pageHeading()).toBe('Restore Job History')
      expect(screen.getByRole('link', { name: /back to restore/i })).toBeInTheDocument()
    })
  })

  describe('index routes still render at the parent path', () => {
    it('/backup renders the backup page', async () => {
      await renderRouteAt('/backup')

      expect(await pageHeading()).toBe('Backup')
      expect(screen.queryByRole('link', { name: /back to targets/i })).not.toBeInTheDocument()
    })

    it('/restore renders the restore page', async () => {
      await renderRouteAt('/restore')

      expect(await pageHeading()).toBe('Restore')
      expect(screen.queryByRole('link', { name: /back to restore/i })).not.toBeInTheDocument()
    })
  })

  // `routes/config/` and `routes/jobs/` have the same directory shape but no
  // flat parent module, so they are siblings under the root rather than a
  // parent/child pair. They were never affected — these lock that in.
  describe('the rest of the tree', () => {
    it.each([
      ['/config', 'Configuration Management'],
      ['/config/storages', 'Storage Management'],
      ['/config/targets', 'Target Management'],
      ['/config/notifiers', 'Notifier Management'],
      ['/config/restore-targets', 'Restore Target Management'],
      ['/config/import', 'Import Configuration'],
      ['/jobs', 'All Jobs'],
    ])('%s renders its own page', async (path, heading) => {
      await renderRouteAt(path)

      expect(await pageHeading()).toBe(heading)
    })
  })
})
