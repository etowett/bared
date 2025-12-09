/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '../test/utils'
import userEvent from '@testing-library/user-event'
import { Dashboard } from './Dashboard'
import * as apiClient from '../api/client'
import * as useDashboardHook from '../hooks/useDashboard'
import * as useJobsHook from '../hooks/useJobs'

// Mock API client
vi.mock('../api/client', () => ({
  clearAuth: vi.fn(),
}))

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
  const mockOnLogout = vi.fn()

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

    render(<Dashboard onLogout={mockOnLogout} />)

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

    render(<Dashboard onLogout={mockOnLogout} />)

    expect(screen.getByText(/BareD - Backup Dashboard/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /logout/i })).toBeInTheDocument()
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

    render(<Dashboard onLogout={mockOnLogout} />)

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

      const { unmount } = render(<Dashboard onLogout={mockOnLogout} />)
      expect(screen.getByText(expected)).toBeInTheDocument()
      unmount()
    })
  })

  it('handles logout correctly', async () => {
    const user = userEvent.setup()
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard onLogout={mockOnLogout} />)

    const logoutButton = screen.getByRole('button', { name: /logout/i })
    await user.click(logoutButton)

    expect(apiClient.clearAuth).toHaveBeenCalled()
    expect(mockOnLogout).toHaveBeenCalled()
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

    render(<Dashboard onLogout={mockOnLogout} />)

    expect(screen.getByText(/loading jobs/i)).toBeInTheDocument()
    expect(screen.queryByTestId('job-list')).not.toBeInTheDocument()
  })

  it('renders status filter dropdown with correct options', () => {
    vi.spyOn(useDashboardHook, 'useDashboard').mockReturnValue({
      data: mockDashboardData,
      isLoading: false,
    } as any)
    vi.spyOn(useJobsHook, 'useJobs').mockReturnValue({
      data: mockJobsData,
      isLoading: false,
    } as any)

    render(<Dashboard onLogout={mockOnLogout} />)

    const filterSelect = screen.getByRole('combobox')
    expect(filterSelect).toBeInTheDocument()

    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(6)
    expect(screen.getByRole('option', { name: /all status/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /queued/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /running/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /completed/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /failed/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /cancelled/i })).toBeInTheDocument()
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

    render(<Dashboard onLogout={mockOnLogout} />)

    // Initially called with undefined (no filter)
    expect(mockUseJobs).toHaveBeenCalledWith(undefined)

    const filterSelect = screen.getByRole('combobox')
    await user.selectOptions(filterSelect, 'running')

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

    render(<Dashboard onLogout={mockOnLogout} />)

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

    render(<Dashboard onLogout={mockOnLogout} />)

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

    render(<Dashboard onLogout={mockOnLogout} />)

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

    render(<Dashboard onLogout={mockOnLogout} />)

    const zeros = screen.getAllByText('0')
    expect(zeros.length).toBeGreaterThanOrEqual(3) // Multiple zeros for stats
    expect(screen.getByText('N/A')).toBeInTheDocument() // Storage
    expect(screen.getByText(/TargetList - 0 targets/)).toBeInTheDocument()
    expect(screen.getByText(/JobList - 0 jobs/)).toBeInTheDocument()
  })
})
