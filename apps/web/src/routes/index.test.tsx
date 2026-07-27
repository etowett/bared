/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen, within } from '@/test/utils'
import type { Dashboard, Target } from '@/types'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('@/hooks/useDashboard', () => ({ useDashboard: vi.fn() }))

vi.mock('@/hooks/useJobs', () => ({
  useJobs: vi.fn(),
  useTriggerBackup: vi.fn(),
}))

import * as useDashboardHook from '@/hooks/useDashboard'
import * as useJobsHook from '@/hooks/useJobs'
import { OverviewPage } from './index'

const target = (overrides: Partial<Target> = {}): Target => ({
  name: 'app-db',
  type: 'postgres',
  database: 'app',
  is_running: false,
  ...overrides,
})

const dashboard = (overrides: Partial<Dashboard> = {}): Dashboard => ({
  targets: [],
  active_jobs: 0,
  total_jobs: 0,
  ...overrides,
})

function setup(data: Dashboard | undefined, query: Record<string, unknown> = {}) {
  vi.mocked(useDashboardHook.useDashboard).mockReturnValue({
    data,
    isPending: data === undefined,
    isError: false,
    error: null,
    refetch: vi.fn(),
    ...query,
  } as any)

  return render(<OverviewPage />)
}

/** The metric figure labels itself via `aria-labelledby`, so this reads the number. */
function metric(label: string) {
  return screen.getByLabelText(label).textContent
}

describe('OverviewPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useJobsHook.useJobs).mockReturnValue({
      data: { jobs: [], total: 0 },
      isPending: false,
    } as any)
    vi.mocked(useJobsHook.useTriggerBackup).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as any)
  })

  describe('unknown is not zero', () => {
    it('renders a null success rate as unknown, never as 0%', () => {
      setup(
        dashboard({
          targets: [target({ last_backup_status: 'success' })],
          success_rate_24h: undefined,
          success_rate_7d: undefined,
          failed_jobs_24h: 0,
        })
      )

      expect(screen.queryByText('0%')).not.toBeInTheDocument()
      expect(screen.getAllByText('unknown').length).toBeGreaterThan(0)
      expect(screen.getByText(/no backup finished in the last 24 hours/i)).toBeInTheDocument()
      expect(screen.getByText(/needs a persistent job store/i)).toBeInTheDocument()
    })

    it('renders a measured 0% as 0%', () => {
      setup(
        dashboard({
          targets: [target({ last_backup_status: 'failed', consecutive_failures: 1 })],
          success_rate_24h: 0,
          failed_jobs_24h: 2,
        })
      )

      expect(screen.getByText('0%')).toBeInTheDocument()
      expect(metric('Failed in 24h')).toBe('2')
    })

    it('says the history was truncated when even the failure count is missing', () => {
      setup(dashboard({ targets: [target({ last_backup_status: 'success' })] }))

      expect(screen.getAllByText(/job history was truncated/i).length).toBeGreaterThan(0)
      expect(metric('Failed in 24h')).toBe('unknown')
    })

    it('renders an absent total_storage_bytes as unavailable, not N/A and not 0', () => {
      setup(dashboard({ targets: [target({ last_backup_status: 'success' })] }))

      expect(screen.getByText('unavailable')).toBeInTheDocument()
      expect(screen.queryByText('N/A')).not.toBeInTheDocument()
      expect(screen.getByText(/does not total stored bytes/i)).toBeInTheDocument()
    })
  })

  describe('attention banner', () => {
    it('surfaces a target with a failure streak and links to its row', () => {
      setup(
        dashboard({
          targets: [
            target({ name: 'orders-db', last_backup_status: 'failed', consecutive_failures: 3 }),
            target({ name: 'quiet-db', last_backup_status: 'success' }),
          ],
        })
      )

      const banner = screen.getByRole('heading', { name: /1 target needs attention/i })
        .parentElement as HTMLElement

      expect(within(banner).getByText('orders-db')).toBeInTheDocument()
      expect(within(banner).getByText('3 failed runs in a row')).toBeInTheDocument()
      expect(within(banner).getByRole('link', { name: /orders-db/i })).toHaveAttribute(
        'href',
        '#target-orders-db'
      )
      expect(metric('Healthy targets')).toBe('1')
    })

    it('lists overdue and never-run targets alongside failures', () => {
      setup(
        dashboard({
          targets: [
            target({ name: 'never-db', last_backup_status: 'never' }),
            target({ name: 'late-db', last_backup_status: 'success', overdue: true }),
          ],
        })
      )

      expect(screen.getByRole('heading', { name: /2 targets need attention/i })).toBeInTheDocument()
      expect(screen.getByText('never backed up')).toBeInTheDocument()
      expect(metric('Overdue')).toBe('1')
    })

    it('confirms everything is current only when every target reported', () => {
      setup(dashboard({ targets: [target({ last_backup_status: 'success' })] }))

      expect(screen.getByText(/backed up and on schedule/i)).toBeInTheDocument()
    })
  })

  describe('absent optional fields never read as healthy', () => {
    it('counts a target with no health fields as unreported, not healthy', () => {
      setup(dashboard({ targets: [target({ name: 'legacy-db' })] }))

      expect(metric('Healthy targets')).toBe('0')
      expect(screen.getByText(/1 report(s)? no health data/i)).toBeInTheDocument()
      expect(screen.queryByText('Healthy')).not.toBeInTheDocument()
      expect(screen.queryByText(/backed up and on schedule/i)).not.toBeInTheDocument()
    })

    it('renders unknown rather than a number for unrecorded size and failure count', () => {
      setup(dashboard({ targets: [target({ name: 'legacy-db' })] }))

      const row = document.getElementById('target-legacy-db') as HTMLElement
      // Last success and last size are both unreported: "unknown", not "never"
      // and not "0 B".
      expect(within(row).getAllByText('unknown')).toHaveLength(2)
      expect(within(row).queryByText('0 B')).not.toBeInTheDocument()
      expect(within(row).queryByText('never')).not.toBeInTheDocument()
    })
  })

  describe('empty state', () => {
    it('invites the operator to configure a target', () => {
      setup(dashboard())

      expect(screen.getByText(/no targets configured$/i)).toBeInTheDocument()
      expect(screen.getByRole('link', { name: /configure targets/i })).toBeInTheDocument()
      expect(screen.getByText(/no target has a schedule/i)).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: /need(s)? attention/i })).not.toBeInTheDocument()
      expect(metric('Healthy targets')).toBe('0')
    })
  })

  describe('loading', () => {
    it('shows a layout-preserving skeleton on the first load only', () => {
      const { unmount } = setup(undefined)

      expect(screen.getByTestId('overview-skeleton')).toBeInTheDocument()
      unmount()

      // A background poll keeps the numbers on screen rather than flashing grey.
      setup(dashboard({ targets: [target({ last_backup_status: 'success' })] }), {
        isFetching: true,
      })

      expect(screen.queryByTestId('overview-skeleton')).not.toBeInTheDocument()
      expect(metric('Healthy targets')).toBe('1')
    })
  })

  describe('in flight', () => {
    it('shows progress for a running job', () => {
      vi.mocked(useJobsHook.useJobs).mockReturnValue({
        data: {
          jobs: [
            {
              id: 'job-1',
              type: 'backup',
              target: 'app-db',
              status: 'running',
              created_at: '2026-07-28T11:00:00Z',
              manual: false,
              progress: {
                stage: 'Uploading',
                percent: 42,
                bytes_processed: 1024,
                bytes_total: 4096,
                message: '',
              },
            },
          ],
          total: 1,
        },
        isPending: false,
      } as any)

      setup(dashboard({ targets: [target({ is_running: true })], active_jobs: 2 }))

      expect(screen.getByText('Uploading')).toBeInTheDocument()
      expect(screen.getByText('42.0%')).toBeInTheDocument()
      expect(screen.getByText(/1 running, 1 queued/i)).toBeInTheDocument()
      expect(metric('Running or queued')).toBe('2')
    })
  })

  describe('error', () => {
    it('explains the failure and offers a retry', () => {
      const refetch = vi.fn()
      vi.mocked(useDashboardHook.useDashboard).mockReturnValue({
        data: undefined,
        isPending: false,
        isError: true,
        error: new Error('boom'),
        refetch,
      } as any)

      render(<OverviewPage />)

      expect(screen.getByText(/could not load the dashboard: boom/i)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
    })
  })
})
