/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as useJobsHook from '../hooks/useJobs'
import { render, screen, waitFor } from '../test/utils'
import type { Job } from '../types'

// Create hoisted mocks that can be used in factory functions
const { mockToastSuccess, mockToastError, mockConfirm } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockConfirm: vi.fn(),
}))

// Mock the useJobs hook
vi.mock('../hooks/useJobs', () => ({
  useCancelJob: vi.fn(),
}))

// Mock JobProgress component
vi.mock('./JobProgress', () => ({
  JobProgress: ({ progress, compact }: { progress: any; compact: boolean }) => (
    <div data-testid="job-progress">
      {progress.percent}% - {compact ? 'compact' : 'full'}
    </div>
  ),
}))

// Mock sonner toast
vi.mock('sonner', () => ({
  toast: {
    success: mockToastSuccess,
    error: mockToastError,
  },
}))

// Stub the confirmation prompt but keep the real ConfirmProvider — the test
// wrapper mounts it, exactly as `__root.tsx` does.
//
// The router is *not* stubbed: the wrapper supplies a real one, so the row's
// primary cell renders a real `<Link>` and the href it produces is the thing
// under test.
vi.mock('@/contexts/ConfirmContext', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/ConfirmContext')>()),
  useConfirm: () => mockConfirm,
}))

// Import after mocks
import { JobList } from './JobList'

/** The row's one visible control for everything that is not "open the job". */
const actionsTrigger = (job: Job) =>
  screen.getByRole('button', { name: `Actions for job ${job.id}` })

async function openMenu(user: ReturnType<typeof userEvent.setup>, job: Job) {
  await user.click(actionsTrigger(job))
  return screen.findByRole('menu')
}

describe('JobList Component', () => {
  const mockOnSelectJob = vi.fn()
  const mockCancelJob = {
    mutateAsync: vi.fn(),
    isPending: false,
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(useJobsHook, 'useCancelJob').mockReturnValue(mockCancelJob as any)
    mockConfirm.mockResolvedValue(true)
  })

  const createMockJob = (overrides: Partial<Job> = {}): Job => ({
    id: '12345678-1234-1234-1234-123456789012',
    type: 'backup',
    target: 'test-db',
    status: 'completed',
    created_at: '2025-12-09T12:00:00Z',
    started_at: '2025-12-09T12:00:05Z',
    completed_at: '2025-12-09T12:05:00Z',
    duration_seconds: 295,
    manual: false,
    ...overrides,
  })

  it('renders empty state when no jobs are provided', () => {
    render(<JobList jobs={[]} onSelectJob={mockOnSelectJob} />)
    expect(screen.getByText(/no jobs found/i)).toBeInTheDocument()
  })

  it('renders job table with correct headers', () => {
    const jobs = [createMockJob()]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText('ID')).toBeInTheDocument()
    expect(screen.getByText('Type')).toBeInTheDocument()
    expect(screen.getByText('Target')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
    expect(screen.getByText('Progress')).toBeInTheDocument()
    expect(screen.getByText('Created')).toBeInTheDocument()
    expect(screen.getByText('Duration')).toBeInTheDocument()
    expect(screen.getByText('Actions')).toBeInTheDocument()
  })

  it('renders job information correctly', () => {
    const jobs = [createMockJob()]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText(/12345678/)).toBeInTheDocument()
    expect(screen.getByText('backup')).toBeInTheDocument()
    expect(screen.getByText('test-db')).toBeInTheDocument()
  })

  it('applies correct status classes', () => {
    const jobs = [
      createMockJob({ id: '1', status: 'running' }),
      createMockJob({ id: '2', status: 'completed' }),
      createMockJob({ id: '3', status: 'failed' }),
    ]
    const { container } = render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    // StatusBadge component should be rendered for each job (3 job rows)
    const rows = container.querySelectorAll('tbody tr')
    expect(rows).toHaveLength(3)
  })

  it('displays manual badge for manual jobs', () => {
    const jobs = [createMockJob({ manual: true })]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText('Manual')).toBeInTheDocument()
  })

  it('renders JobProgress component when progress is available', () => {
    const jobs = [
      createMockJob({
        progress: { percent: 50, current: 50, total: 100 },
      }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByTestId('job-progress')).toBeInTheDocument()
    expect(screen.getByText('50% - compact')).toBeInTheDocument()
  })

  it('shows dash when progress is not available', () => {
    const jobs = [createMockJob({ progress: undefined })]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    const progressCell = screen.getByText('-')
    expect(progressCell).toBeInTheDocument()
  })

  it('formats dates correctly', () => {
    const jobs = [createMockJob({ created_at: '2025-12-09T13:00:00Z' })]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    // Twice: the Created column, plus the copy folded into the id cell for the
    // widths where that column is hidden. jsdom applies no media queries, so
    // both are in the DOM here.
    expect(screen.getAllByText(/12\/9\/2025/)).toHaveLength(2)
  })

  it('formats duration correctly for different time ranges', () => {
    const jobs = [
      createMockJob({ id: '1', duration_seconds: 30 }),
      createMockJob({ id: '2', duration_seconds: 120 }),
      createMockJob({ id: '3', duration_seconds: 3600 }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    // Check that durations are formatted
    expect(screen.getByText('30s')).toBeInTheDocument()
    expect(screen.getByText('2m 0s')).toBeInTheDocument()
    expect(screen.getByText('1h 0m 0s')).toBeInTheDocument()
  })

  it('calls onSelectJob when row is clicked', async () => {
    const user = userEvent.setup()
    const job = createMockJob()

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    const row = screen.getByText(/12345678/).closest('tr')
    await user.click(row!)

    expect(mockOnSelectJob).toHaveBeenCalledWith(job)
  })

  describe('keyboard access', () => {
    it('makes the id cell focusable in dialog mode', async () => {
      const user = userEvent.setup()
      const job = createMockJob()

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      const trigger = screen.getByRole('button', { name: /job details for 12345678/i })
      await user.tab()
      expect(trigger).toHaveFocus()
    })

    it('opens a job with Enter', async () => {
      const user = userEvent.setup()
      const job = createMockJob()

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      screen.getByRole('button', { name: /job details for 12345678/i }).focus()
      await user.keyboard('{Enter}')

      expect(mockOnSelectJob).toHaveBeenCalledWith(job)
    })

    it('opens a job with Space', async () => {
      const user = userEvent.setup()
      const job = createMockJob()

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      screen.getByRole('button', { name: /job details for 12345678/i }).focus()
      await user.keyboard(' ')

      expect(mockOnSelectJob).toHaveBeenCalledWith(job)
    })

    it('uses a real link in navigation mode, so copy-link and new-tab work', () => {
      const job = createMockJob()

      render(<JobList jobs={[job]} navigationMode />)

      const link = screen.getByRole('link', { name: /job 12345678/i })
      expect(link).toHaveAttribute('href', `/jobs/${job.id}`)
    })

    it('keeps the actions trigger out of the row control', async () => {
      const user = userEvent.setup()
      const job = createMockJob({ status: 'running' })

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      const trigger = actionsTrigger(job)
      // A button nested inside the row's link/button would be invalid HTML and
      // would swallow the row's own activation.
      expect(trigger.closest('a')).toBeNull()
      expect(trigger.closest('button')).toBe(trigger)

      await user.click(trigger)
      expect(mockOnSelectJob).not.toHaveBeenCalled()
    })
  })

  describe('row overflow menu', () => {
    it('hides the destructive action behind one neutral trigger', async () => {
      const user = userEvent.setup()
      const job = createMockJob({ status: 'running' })

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      // Nothing destructive is visible until the menu is opened.
      expect(screen.queryByRole('menuitem', { name: /cancel job/i })).not.toBeInTheDocument()

      await openMenu(user, job)
      expect(await screen.findByRole('menuitem', { name: /cancel job/i })).toBeInTheDocument()
    })

    it('is keyboard operable end to end', async () => {
      const user = userEvent.setup()
      const job = createMockJob({ status: 'running' })
      mockCancelJob.mutateAsync.mockResolvedValueOnce(undefined)

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      actionsTrigger(job).focus()
      await user.keyboard('{Enter}')

      const menu = await screen.findByRole('menu')
      expect(menu).toBeInTheDocument()

      // Arrow down past "View details" and "Copy job ID" to the cancel item.
      await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}{Enter}')

      await waitFor(() => {
        expect(mockConfirm).toHaveBeenCalled()
        expect(mockCancelJob.mutateAsync).toHaveBeenCalledWith(job.id)
      })
    })

    it('offers no cancel item for a finished job', async () => {
      const user = userEvent.setup()
      const job = createMockJob({ status: 'completed' })

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      await openMenu(user, job)

      expect(screen.queryByRole('menuitem', { name: /cancel job/i })).not.toBeInTheDocument()
      expect(screen.getByRole('menuitem', { name: /view details/i })).toBeInTheDocument()
    })

    it('opens the job from the menu', async () => {
      const user = userEvent.setup()
      const job = createMockJob()

      render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

      await openMenu(user, job)
      await user.click(screen.getByRole('menuitem', { name: /view details/i }))

      expect(mockOnSelectJob).toHaveBeenCalledWith(job)
    })
  })

  describe('sorting', () => {
    const jobs = [
      createMockJob({ id: 'a', target: 'zeta', duration_seconds: 10 }),
      createMockJob({ id: 'b', target: 'alpha', duration_seconds: 90 }),
    ]

    it('leaves the headers plain when the caller does not handle sorting', () => {
      render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

      expect(screen.queryByRole('button', { name: /target/i })).not.toBeInTheDocument()
    })

    it('reports the active column through aria-sort', () => {
      render(
        <JobList
          jobs={jobs}
          onSelectJob={mockOnSelectJob}
          sort="target"
          order="asc"
          onSortChange={vi.fn()}
        />
      )

      expect(screen.getByRole('columnheader', { name: /target/i })).toHaveAttribute(
        'aria-sort',
        'ascending'
      )
      expect(screen.getByRole('columnheader', { name: /^status$/i })).toHaveAttribute(
        'aria-sort',
        'none'
      )
    })

    it('reorders the rows it was given', () => {
      const { container } = render(
        <JobList
          jobs={jobs}
          onSelectJob={mockOnSelectJob}
          sort="target"
          order="asc"
          onSortChange={vi.fn()}
        />
      )

      const targets = Array.from(container.querySelectorAll('tbody tr td:nth-child(3)')).map(
        (cell) => cell.textContent
      )
      expect(targets).toEqual(['alpha', 'zeta'])
    })

    it('asks for the opposite direction when the active column is clicked again', async () => {
      const user = userEvent.setup()
      const onSortChange = vi.fn()

      render(
        <JobList
          jobs={jobs}
          onSelectJob={mockOnSelectJob}
          sort="target"
          order="asc"
          onSortChange={onSortChange}
        />
      )

      await user.click(screen.getByRole('button', { name: /target/i }))
      expect(onSortChange).toHaveBeenCalledWith('target', 'desc')
    })
  })

  it('highlights selected job row', () => {
    const job = createMockJob()
    const { container } = render(
      <JobList jobs={[job]} onSelectJob={mockOnSelectJob} selectedJobId={job.id} />
    )

    const row = container.querySelector('tr.bg-primary\\/10')
    expect(row).toBeInTheDocument()
  })

  it('offers cancel for running, queued, and cancelling jobs only', async () => {
    const user = userEvent.setup()
    const jobs = [
      createMockJob({ id: '1', status: 'running' }),
      createMockJob({ id: '2', status: 'queued' }),
      createMockJob({ id: '3', status: 'cancelling' }),
      createMockJob({ id: '4', status: 'completed' }),
      createMockJob({ id: '5', status: 'failed' }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    const cancellable: string[] = []
    for (const job of jobs) {
      await openMenu(user, job)
      if (screen.queryByRole('menuitem', { name: /cancel job/i })) {
        cancellable.push(job.id)
      }
      await user.keyboard('{Escape}')
    }

    expect(cancellable).toEqual(['1', '2', '3'])
  })

  it('handles job cancellation with confirmation', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    mockCancelJob.mutateAsync.mockResolvedValueOnce(undefined)

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    await openMenu(user, job)
    await user.click(screen.getByRole('menuitem', { name: /cancel job/i }))

    expect(mockConfirm).toHaveBeenCalledWith({
      title: 'Cancel Job',
      description: 'Are you sure you want to cancel this job?',
      confirmLabel: 'Cancel Job',
      cancelLabel: 'Keep Running',
      variant: 'destructive',
    })
    await waitFor(() => {
      expect(mockCancelJob.mutateAsync).toHaveBeenCalledWith(job.id)
      expect(mockToastSuccess).toHaveBeenCalledWith('Job cancellation requested')
    })
  })

  it('does not cancel job if confirmation is denied', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    mockConfirm.mockResolvedValueOnce(false)

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    await openMenu(user, job)
    await user.click(screen.getByRole('menuitem', { name: /cancel job/i }))

    await waitFor(() => expect(mockConfirm).toHaveBeenCalled())
    expect(mockCancelJob.mutateAsync).not.toHaveBeenCalled()
  })

  it('shows alert on cancellation error', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    mockCancelJob.mutateAsync.mockRejectedValueOnce(new Error('Network error'))

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    await openMenu(user, job)
    await user.click(screen.getByRole('menuitem', { name: /cancel job/i }))

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith('Failed to cancel job', {
        description: 'Network error',
      })
    })
  })

  it('prevents row selection when the menu is opened', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    await openMenu(user, job)

    // onSelectJob should not be called because stopPropagation prevents it
    expect(mockOnSelectJob).not.toHaveBeenCalled()
  })

  it('disables the cancel item when the job is already cancelling', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'cancelling' })
    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    await openMenu(user, job)

    expect(screen.getByRole('menuitem', { name: /cancel job/i })).toHaveAttribute(
      'data-disabled',
      ''
    )
  })

  it('disables the cancel item while a cancel is in flight', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    mockCancelJob.isPending = true
    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    await openMenu(user, job)

    expect(screen.getByRole('menuitem', { name: /cancel job/i })).toHaveAttribute(
      'data-disabled',
      ''
    )
  })
})
