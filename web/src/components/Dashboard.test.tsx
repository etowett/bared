/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as useDashboardHook from '../hooks/useDashboard'
import * as useJobsHook from '../hooks/useJobs'
import { render, screen, waitFor } from '../test/utils'
import { Dashboard } from './Dashboard'

// Mock hooks
vi.mock('../hooks/useDashboard', () => ({
  useDashboard: vi.fn(),
}))

vi.mock('../hooks/useJobs', () => ({
  useJobs: vi.fn(),
}))

// Mock child components
vi.mock('./RestoreForm', () => ({
  RestoreForm: () => <div data-testid="restore-form">RestoreForm</div>,
}))

vi.mock('./TargetList', () => ({
  TargetList: ({ targets }: any) => (
    <div data-testid="target-list">TargetList - {targets.length} targets</div>
  ),
}))

vi.mock('./JobList', () => ({
  JobList: ({ jobs, onSelectJob }: any) => (
    <div data-testid="job-list">
      JobList - {jobs.length} jobs
      <button onClick={() => onSelectJob(jobs[0])}>Select First Job</button>
    </div>
  ),
}))

vi.mock('./JobDetail', () => ({
  JobDetail: ({ job, onClose }: any) => (
    <div data-testid="job-detail">
      JobDetail - {job.id}
      <button onClick={onClose}>Close</button>
    </div>
  ),
}))

describe('Dashboard Component', () => {
  const mockDashboardData = {
    targets: [
      { name: 'db1', type: 'mysql', database: 'test', is_running: false },
      { name: 'db2', type: 'postgres', database: 'prod', is_running: true },
    ],
    active_jobs: 3,
    total_jobs: 150,
    total_storage_bytes: 1073741824, // 1 GB
  }

  const mockJobsData = {
    jobs: [
      {
        id: 'job1',
        type: 'backup' as const,
        target: 'db1',
        status: 'running' as const,
        created_at: '2025-12-09T10:00:00Z',
        manual: false,
      },
      {
        id: 'job2',
        type: 'restore' as const,
        target: 'db2',
        status: 'completed' as const,
        created_at: '2025-12-09T09:00:00Z',
        manual: true,
      },
    ],
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading state while dashboard is loading', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: undefined,
      isLoading: true,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: undefined,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    expect(screen.getByText(/loading dashboard/i)).toBeInTheDocument()
  })

  it('renders dashboard with all sections after loading', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    // Check for section headings and components
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByTestId('restore-form')).toBeInTheDocument()
    expect(screen.getByTestId('target-list')).toBeInTheDocument()
    expect(screen.getByTestId('job-list')).toBeInTheDocument()
  })

  it('displays correct stats from dashboard data', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    expect(screen.getByText('Targets')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument() // 2 targets

    expect(screen.getByText('Active Jobs')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument() // 3 active jobs

    expect(screen.getByText('Total Jobs')).toBeInTheDocument()
    expect(screen.getByText('150')).toBeInTheDocument() // 150 total jobs

    expect(screen.getByText('Storage Used')).toBeInTheDocument()
    expect(screen.getByText('1.00 GB')).toBeInTheDocument() // 1 GB
  })

  it('formats bytes correctly for different sizes', () => {
    const testCases = [
      { bytes: undefined, expected: 'N/A' },
      { bytes: 500, expected: '500.00 B' },
      { bytes: 1024, expected: '1.00 KB' },
      { bytes: 1048576, expected: '1.00 MB' },
      { bytes: 1073741824, expected: '1.00 GB' },
      { bytes: 1099511627776, expected: '1.00 TB' },
    ]

    testCases.forEach(({ bytes, expected }) => {
      vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
        data: { ...mockDashboardData, total_storage_bytes: bytes },
        isLoading: false,
      } as any)
      vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
        data: mockJobsData,
        isLoading: false,
      } as any)

      const { unmount } = render(<Dashboard />)
      expect(screen.getByText(expected)).toBeInTheDocument()
      unmount()
    })
  })

  it('shows loading state for jobs section', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: undefined,
      isLoading: true,
    } as any)

    render(<Dashboard />)

    expect(screen.getByText(/loading jobs/i)).toBeInTheDocument()
    expect(screen.queryByTestId('job-list')).not.toBeInTheDocument()
  })

  it('renders status filter dropdown', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    // Verify the filter combobox is rendered
    const filterSelect = screen.getByRole('combobox')
    expect(filterSelect).toBeInTheDocument()
  })

  it('filters jobs by status when filter is changed', async () => {
    const user = userEvent.setup()
    const mockUseJobs = vi.spyOn(useJobsHook, 'useJobs')

    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    mockUseJobs.mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    // Initially called with undefined (no filter)
    expect(mockUseJobs).toHaveBeenCalledWith(undefined)

    const filterSelect = screen.getByRole('combobox')

    // Open the select
    await user.click(filterSelect)

    // Wait for and click the "running" option
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /running/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /running/i }))

    // Should be called with status filter
    await waitFor(() => {
      expect(mockUseJobs).toHaveBeenCalledWith({ status: 'running' })
    })
  })

  it('opens job detail modal when job is selected', async () => {
    const user = userEvent.setup()
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    expect(screen.queryByTestId('job-detail')).not.toBeInTheDocument()

    const selectButton = screen.getByText('Select First Job')
    await user.click(selectButton)

    await waitFor(() => {
      expect(screen.getByTestId('job-detail')).toBeInTheDocument()
      expect(screen.getByText(/JobDetail - job1/)).toBeInTheDocument()
    })
  })

  it('closes job detail modal when close is clicked', async () => {
    const user = userEvent.setup()
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    // Open modal
    await user.click(screen.getByText('Select First Job'))
    expect(screen.getByTestId('job-detail')).toBeInTheDocument()

    // Close modal
    await user.click(screen.getByText('Close'))

    await waitFor(() => {
      expect(screen.queryByTestId('job-detail')).not.toBeInTheDocument()
    })
  })

  it('passes correct props to child components', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    expect(screen.getByText(/TargetList - 2 targets/)).toBeInTheDocument()
    expect(screen.getByText(/JobList - 2 jobs/)).toBeInTheDocument()
  })

  it('handles missing dashboard data gracefully', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: undefined,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: undefined,
      isLoading: false,
    } as any)

    render(<Dashboard />)

    const zeros = screen.getAllByText('0')
    expect(zeros.length).toBeGreaterThanOrEqual(3) // Multiple zeros for stats
    expect(screen.getByText('N/A')).toBeInTheDocument() // Storage
    expect(screen.getByText(/TargetList - 0 targets/)).toBeInTheDocument()
    expect(screen.getByText(/JobList - 0 jobs/)).toBeInTheDocument()
  })
})
