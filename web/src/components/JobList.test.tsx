/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '../test/utils'
import userEvent from '@testing-library/user-event'
import { JobList } from './JobList'
import type { Job } from '../types'
import * as useJobsHook from '../hooks/useJobs'

// Mock the useJobs hook
vi.mock('../hooks/useJobs', () => ({
  useCancelJob: vi.fn(),
}))

// Mock JobProgress component
vi.mock('./JobProgress', () => ({
  JobProgress: ({ progress, compact }: { progress: any; compact: boolean }) => (
    <div data-testid="job-progress">{progress.percent}% - {compact ? 'compact' : 'full'}</div>
  ),
}))

describe('JobList Component', () => {
  const mockOnSelectJob = vi.fn()
  const mockCancelJob = {
    mutateAsync: vi.fn(),
    isPending: false,
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(useJobsHook, 'useCancelJob').mockReturnValue(mockCancelJob as any)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'alert').mockImplementation(() => {})
  })

  const createMockJob = (overrides: Partial<Job> = {}): Job => ({
    id: '12345678-1234-1234-1234-123456789012',
    type: 'backup',
    target: 'test-db',
    status: 'completed',
    created_at: '2025-12-09T10:00:00Z',
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
    const jobs = [
      createMockJob({
        id: '12345678-abcd-1234-5678-123456789012',
        type: 'backup',
        target: 'prod-database',
        status: 'completed',
      }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText('12345678')).toBeInTheDocument() // Short ID
    expect(screen.getByText('backup')).toBeInTheDocument()
    expect(screen.getByText('prod-database')).toBeInTheDocument()
    expect(screen.getByText('completed')).toBeInTheDocument()
  })

  it('applies correct status classes', () => {
    const jobs = [
      createMockJob({ id: '1', status: 'queued' }),
      createMockJob({ id: '2', status: 'running' }),
      createMockJob({ id: '3', status: 'completed' }),
      createMockJob({ id: '4', status: 'failed' }),
      createMockJob({ id: '5', status: 'cancelled' }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText('queued').className).toContain('status-queued')
    expect(screen.getByText('running').className).toContain('status-running')
    expect(screen.getByText('completed').className).toContain('status-completed')
    expect(screen.getByText('failed').className).toContain('status-failed')
    expect(screen.getByText('cancelled').className).toContain('status-cancelled')
  })

  it('displays manual badge for manual jobs', () => {
    const jobs = [
      createMockJob({ id: 'job1', manual: true }),
      createMockJob({ id: 'job2', manual: false }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    const manualBadges = screen.getAllByText('Manual')
    expect(manualBadges).toHaveLength(1)
  })

  it('renders JobProgress component when progress is available', () => {
    const jobs = [
      createMockJob({
        progress: {
          stage: 'uploading',
          percent: 75,
          bytes_processed: 750000,
          bytes_total: 1000000,
          message: 'Uploading...',
        },
      }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByTestId('job-progress')).toBeInTheDocument()
    expect(screen.getByText(/75%/)).toBeInTheDocument()
    expect(screen.getByText(/compact/)).toBeInTheDocument()
  })

  it('shows dash when progress is not available', () => {
    const jobs = [createMockJob({ progress: undefined })]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText('-')).toBeInTheDocument()
  })

  it('formats dates correctly', () => {
    const jobs = [
      createMockJob({
        created_at: '2025-12-09T10:00:00Z',
      }),
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    // Date formatting depends on locale, just check it's not the raw string
    const dateCell = screen.getByText(/12\/9\/2025|2025-12-09/i)
    expect(dateCell).toBeInTheDocument()
  })

  it('formats duration correctly for different time ranges', () => {
    const jobs = [
      createMockJob({ id: '1', duration_seconds: 45 }), // < 1 minute
      createMockJob({ id: '2', duration_seconds: 125 }), // > 1 minute
      createMockJob({ id: '3', duration_seconds: 3665 }), // > 1 hour
      createMockJob({ id: '4', duration_seconds: undefined }), // No duration
    ]
    render(<JobList jobs={jobs} onSelectJob={mockOnSelectJob} />)

    expect(screen.getByText('45s')).toBeInTheDocument()
    expect(screen.getByText('2m 5s')).toBeInTheDocument()
    expect(screen.getByText('1h 1m 5s')).toBeInTheDocument()
    expect(screen.getByText('N/A')).toBeInTheDocument()
  })

  it('calls onSelectJob when row is clicked', async () => {
    const user = userEvent.setup()
    const job = createMockJob()
    render(<JobList jobs={[job]} onSelectJob={mockOnSelectJob} />)

    const row = screen.getByText('12345678').closest('tr')
    await user.click(row!)

    expect(mockOnSelectJob).toHaveBeenCalledWith(job)
  })

  it('highlights selected job row', () => {
    const jobs = [
      createMockJob({ id: 'selected-job-id' }),
      createMockJob({ id: 'other-job-id' }),
    ]
    render(
      <JobList jobs={jobs} onSelectJob={mockOnSelectJob} selectedJobId="selected-job-id" />
    )

    const rows = screen.getAllByRole('row')
    // First row is header, second is selected job
    expect(rows[1].className).toContain('selected')
    expect(rows[2].className).not.toContain('selected')
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

    expect(window.confirm).toHaveBeenCalledWith('Are you sure you want to cancel this job?')
    await waitFor(() => {
      expect(mockCancelJob.mutateAsync).toHaveBeenCalledWith(job.id)
    })
  })

  it('does not cancel job if confirmation is denied', async () => {
    const user = userEvent.setup()
    const job = createMockJob({ status: 'running' })
    vi.spyOn(window, 'confirm').mockReturnValueOnce(false)

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
      expect(window.alert).toHaveBeenCalledWith('Failed to cancel job: Error: Network error')
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
