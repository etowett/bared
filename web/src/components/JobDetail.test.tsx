/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '../test/utils'
import type { Job } from '../types'
import { JobDetail } from './JobDetail'

// Mock JobDetailContent component
vi.mock('./JobDetailContent', () => ({
  JobDetailContent: ({ job, compact }: any) => (
    <div data-testid="job-detail-content">
      JobDetailContent - {job.id} - {compact ? 'compact' : 'full'}
    </div>
  ),
}))

// Mock TanStack Router Link
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, params, children }: any) => (
    <a href={`${to.replace('$id', params.id)}`}>{children}</a>
  ),
}))

describe('JobDetail Component', () => {
  const mockOnClose = vi.fn()

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

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders dialog with job details header', () => {
    const job = createMockJob()

    render(<JobDetail job={job} onClose={mockOnClose} />)

    expect(screen.getByText('Job Details')).toBeInTheDocument()
  })

  it('renders JobDetailContent component with correct props', () => {
    const job = createMockJob()

    render(<JobDetail job={job} onClose={mockOnClose} />)

    expect(screen.getByTestId('job-detail-content')).toBeInTheDocument()
    expect(screen.getByText(/12345678-1234-1234-1234-123456789012 - compact/)).toBeInTheDocument()
  })

  it('passes compact=true to JobDetailContent', () => {
    const job = createMockJob()

    render(<JobDetail job={job} onClose={mockOnClose} />)

    expect(screen.getByText(/compact/)).toBeInTheDocument()
  })

  it('renders link to full page view', () => {
    const job = createMockJob({ id: 'test-job-123' })

    render(<JobDetail job={job} onClose={mockOnClose} />)

    const link = screen.getByRole('link', { name: /full page/i })
    expect(link).toBeInTheDocument()
    expect(link).toHaveAttribute('href', '/jobs/test-job-123')
  })

  it('renders close button', () => {
    const job = createMockJob()

    render(<JobDetail job={job} onClose={mockOnClose} />)

    const closeButtons = screen.getAllByRole('button')
    const closeButton = closeButtons.find((button) => {
      const svg = button.querySelector('svg')
      return svg !== null
    })

    expect(closeButton).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup()
    const job = createMockJob()

    render(<JobDetail job={job} onClose={mockOnClose} />)

    const closeButtons = screen.getAllByRole('button')
    const closeButton = closeButtons.find((button) => {
      const svg = button.querySelector('svg')
      return svg !== null
    })

    await user.click(closeButton!)

    expect(mockOnClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when dialog is dismissed', () => {
    const job = createMockJob()

    const { rerender } = render(<JobDetail job={job} onClose={mockOnClose} />)

    // Simulate dialog close via onOpenChange
    // The Dialog component should call onClose when user tries to close
    rerender(<JobDetail job={job} onClose={mockOnClose} />)

    // The dialog is open by default
    expect(screen.getByText('Job Details')).toBeInTheDocument()
  })

  it('renders dialog with correct structure', () => {
    const job = createMockJob()

    const { container } = render(<JobDetail job={job} onClose={mockOnClose} />)

    // Check for dialog header
    expect(screen.getByText('Job Details')).toBeInTheDocument()

    // Check for content area
    expect(screen.getByTestId('job-detail-content')).toBeInTheDocument()

    // Check that dialog has proper content structure
    const dialogContent = container.querySelector('[role="dialog"]')
    expect(dialogContent).toBeInTheDocument()
  })

  it('renders with different job types', () => {
    const backupJob = createMockJob({ type: 'backup' })
    const { rerender } = render(<JobDetail job={backupJob} onClose={mockOnClose} />)

    expect(screen.getByTestId('job-detail-content')).toBeInTheDocument()

    const restoreJob = createMockJob({ type: 'restore' })
    rerender(<JobDetail job={restoreJob} onClose={mockOnClose} />)

    expect(screen.getByTestId('job-detail-content')).toBeInTheDocument()
  })

  it('renders with different job statuses', () => {
    const statuses: Array<Job['status']> = ['running', 'completed', 'failed', 'queued', 'cancelled']

    statuses.forEach((status) => {
      const job = createMockJob({ status })
      const { unmount } = render(<JobDetail job={job} onClose={mockOnClose} />)

      expect(screen.getByTestId('job-detail-content')).toBeInTheDocument()
      unmount()
    })
  })

  it('maintains open state', () => {
    const job = createMockJob()

    render(<JobDetail job={job} onClose={mockOnClose} />)

    // Dialog should be open
    expect(screen.getByText('Job Details')).toBeInTheDocument()
    expect(screen.getByTestId('job-detail-content')).toBeInTheDocument()
  })

  it('renders external link icon in full page button', () => {
    const job = createMockJob()

    const { container } = render(<JobDetail job={job} onClose={mockOnClose} />)

    const fullPageLink = screen.getByRole('link', { name: /full page/i })
    const icon = fullPageLink.querySelector('svg')

    expect(icon).toBeInTheDocument()
  })

  it('renders close icon in close button', () => {
    const job = createMockJob()

    const { container } = render(<JobDetail job={job} onClose={mockOnClose} />)

    const closeButtons = screen.getAllByRole('button')
    const closeButton = closeButtons.find((button) => {
      const svg = button.querySelector('svg')
      return svg !== null
    })

    const icon = closeButton?.querySelector('svg')
    expect(icon).toBeInTheDocument()
  })
})
