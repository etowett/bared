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
vi.mock('@/contexts/ConfirmContext', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/ConfirmContext')>()),
  useConfirm: () => mockConfirm,
}))

// Import after mocks
import { JobList } from './JobList'

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

    // formatDate should format the date
    expect(screen.getByText(/12\/9\/2025/)).toBeInTheDocument()
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

  it('highlights selected job row', () => {
    const job = createMockJob()
    const { container } = render(
      <JobList jobs={[job]} onSelectJob={mockOnSelectJob} selectedJobId={job.id} />
    )

    const row = container.querySelector('tr.bg-primary\\/10')
    expect(row).toBeInTheDocument()
  })

  it('shows cancel button for running, queued, and cancelling jobs', () => {
    const jobs = [
      createMockJob({ id: '1', status: 'running' }),
      createMockJob({ id: '2', status: 'queued' }),
      createMockJob({ id: '3', status: 'cancelling' }),
      createMockJob({ id: '4', status: 'completed' }),
      createMockJob({ id: '5', status: 'failed' }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    const cancelButtons = screen.getAllByRole('button', { name: /cancel/i })
    expect(cancelButtons).toHaveLength(3)
  })

  it('handles job cancellation with confirmation', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    mockCancelJob.mutateAsync.mockResolvedValueOnce(undefined)

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelButton)

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

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelButton)

    expect(mockCancelJob.mutateAsync).not.toHaveBeenCalled()
  })

  it('shows alert on cancellation error', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    mockCancelJob.mutateAsync.mockRejectedValueOnce(new Error('Network error'))

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelButton)

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith('Failed to cancel job', {
        description: 'Network error',
      })
    })
  })

  it('prevents row selection when cancel button is clicked', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })

    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelButton)

    // onSelectJob should not be called because stopPropagation prevents it
    expect(mockOnSelectJob).not.toHaveBeenCalled()
  })

  it('disables cancel button when job is cancelling', () => {
    const jobs = [createMockJob({ status: 'cancelling' })]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    expect(cancelButton).toBeDisabled()
  })

  it('disables cancel button when cancel mutation is pending', () => {
    const jobs = [createMockJob({ status: 'running' })]
    mockCancelJob.isPending = true
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    expect(cancelButton).toBeDisabled()
  })
})
