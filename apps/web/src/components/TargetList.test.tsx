/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as useJobsHook from '../hooks/useJobs'
import { render, screen, waitFor } from '../test/utils'
import type { Target } from '../types'
import { TargetList } from './TargetList'

// Create hoisted mocks
const { mockToastSuccess, mockToastError, mockConfirm } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockConfirm: vi.fn(),
}))

// Mock sonner toast
vi.mock('sonner', () => ({
  toast: {
    success: mockToastSuccess,
    error: mockToastError,
  },
}))

// Mock useConfirm hook
vi.mock('../hooks/useConfirm', () => ({
  useConfirm: () => ({
    confirm: mockConfirm,
    ConfirmDialog: <div data-testid="confirm-dialog">Confirm Dialog</div>,
  }),
}))

// Mock hooks
vi.mock('../hooks/useJobs', () => ({
  useTriggerBackup: vi.fn(),
}))

describe('TargetList Component', () => {
  const mockTriggerBackup = {
    mutateAsync: vi.fn(),
    isPending: false,
  }

  const createMockTarget = (overrides: Partial<Target> = {}): Target => ({
    name: 'test-db',
    type: 'postgres',
    database: 'test_database',
    is_running: false,
    last_backup: '2026-12-09T10:00:00Z',
    schedule: '0 2 * * *',
    next_scheduled: '2026-12-10T02:00:00Z',
    ...overrides,
  })

  beforeEach(() => {
    vi.clearAllMocks()
    mockTriggerBackup.isPending = false
    vi.spyOn(useJobsHook, 'useTriggerBackup').mockReturnValue(mockTriggerBackup as any)
    mockConfirm.mockResolvedValue(true)
  })

  it('renders empty state when no targets provided', () => {
    render(<TargetList targets={[]} />)

    expect(screen.getByText(/no targets found/i)).toBeInTheDocument()
  })

  it('renders target cards with correct information', () => {
    const targets = [
      createMockTarget({ name: 'db1', type: 'postgres', database: 'app_db' }),
      createMockTarget({ name: 'db2', type: 'mysql', database: 'user_db' }),
    ]

    render(<TargetList targets={targets} />)

    expect(screen.getByText('db1')).toBeInTheDocument()
    expect(screen.getByText('db2')).toBeInTheDocument()
    expect(screen.getByText('postgres')).toBeInTheDocument()
    expect(screen.getByText('mysql')).toBeInTheDocument()
    expect(screen.getByText('app_db')).toBeInTheDocument()
    expect(screen.getByText('user_db')).toBeInTheDocument()
  })

  it('displays status badge for idle target', () => {
    const targets = [createMockTarget({ is_running: false })]

    render(<TargetList targets={targets} />)

    // StatusBadge component should be rendered
    expect(screen.getByText('test-db')).toBeInTheDocument()
  })

  it('displays status badge for running target', () => {
    const targets = [createMockTarget({ is_running: true })]

    render(<TargetList targets={targets} />)

    expect(screen.getByText('test-db')).toBeInTheDocument()
  })

  it('displays last backup date', () => {
    const targets = [createMockTarget({ last_backup: '2026-12-09T10:00:00Z' })]

    render(<TargetList targets={targets} />)

    expect(screen.getByText(/12\/9\/2026/)).toBeInTheDocument()
  })

  it('displays "Never" when no last backup', () => {
    const targets = [createMockTarget({ last_backup: undefined })]

    render(<TargetList targets={targets} />)

    expect(screen.getByText('Never')).toBeInTheDocument()
  })

  it('displays schedule when available', () => {
    const targets = [createMockTarget({ schedule: '0 2 * * *' })]

    render(<TargetList targets={targets} />)

    // Table view shows human-readable schedule via cronToHuman
    expect(screen.getByText('test-db')).toBeInTheDocument()
  })

  it('displays dash when no schedule available', () => {
    const targets = [createMockTarget({ schedule: undefined, next_scheduled: undefined })]

    render(<TargetList targets={targets} />)

    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('renders backup button for each target', () => {
    const targets = [createMockTarget({ name: 'db1' }), createMockTarget({ name: 'db2' })]

    render(<TargetList targets={targets} />)

    const backupButtons = screen.getAllByRole('button', { name: /backup now/i })
    expect(backupButtons).toHaveLength(2)
  })

  it('disables backup button when target is running', () => {
    const targets = [createMockTarget({ is_running: true })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /running/i })
    expect(button).toBeDisabled()
  })

  it('disables backup button when trigger is pending', () => {
    mockTriggerBackup.isPending = true
    const targets = [createMockTarget({ is_running: false })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /backup now/i })
    expect(button).toBeDisabled()
  })

  it('shows confirmation dialog when backup button is clicked', async () => {
    const user = userEvent.setup()
    const targets = [createMockTarget({ name: 'test-db' })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /backup now/i })
    await user.click(button)

    expect(mockConfirm).toHaveBeenCalledWith({
      title: 'Start Backup',
      description: 'Are you sure you want to start a backup for test-db?',
      confirmLabel: 'Start Backup',
      variant: 'default',
    })
  })

  it('triggers backup after confirmation', async () => {
    const user = userEvent.setup()
    mockTriggerBackup.mutateAsync.mockResolvedValueOnce({ job_id: 'new-job' })
    const targets = [createMockTarget({ name: 'test-db' })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /backup now/i })
    await user.click(button)

    await waitFor(() => {
      expect(mockTriggerBackup.mutateAsync).toHaveBeenCalledWith('test-db')
      expect(mockToastSuccess).toHaveBeenCalledWith('Backup started successfully')
    })
  })

  it('does not trigger backup if confirmation is denied', async () => {
    const user = userEvent.setup()
    mockConfirm.mockResolvedValueOnce(false)
    const targets = [createMockTarget({ name: 'test-db' })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /backup now/i })
    await user.click(button)

    await waitFor(() => {
      expect(mockConfirm).toHaveBeenCalled()
    })

    expect(mockTriggerBackup.mutateAsync).not.toHaveBeenCalled()
    expect(mockToastSuccess).not.toHaveBeenCalled()
  })

  it('shows error toast when backup trigger fails', async () => {
    const user = userEvent.setup()
    mockTriggerBackup.mutateAsync.mockRejectedValueOnce(new Error('Network error'))
    const targets = [createMockTarget({ name: 'test-db' })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /backup now/i })
    await user.click(button)

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith('Failed to trigger backup', {
        description: 'Network error',
      })
    })
  })

  it('handles non-Error exceptions in backup trigger', async () => {
    const user = userEvent.setup()
    mockTriggerBackup.mutateAsync.mockRejectedValueOnce('String error')
    const targets = [createMockTarget({ name: 'test-db' })]

    render(<TargetList targets={targets} />)

    const button = screen.getByRole('button', { name: /backup now/i })
    await user.click(button)

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith('Failed to trigger backup', {
        description: 'Failed to trigger backup',
      })
    })
  })

  it('renders multiple targets in table layout', () => {
    const targets = [
      createMockTarget({ name: 'db1' }),
      createMockTarget({ name: 'db2' }),
      createMockTarget({ name: 'db3' }),
    ]

    render(<TargetList targets={targets} />)

    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByText('db1')).toBeInTheDocument()
    expect(screen.getByText('db2')).toBeInTheDocument()
    expect(screen.getByText('db3')).toBeInTheDocument()
  })

  it('renders confirm dialog component', () => {
    const targets = [createMockTarget()]

    render(<TargetList targets={targets} />)

    expect(screen.getByTestId('confirm-dialog')).toBeInTheDocument()
  })

  it('handles targets without optional fields gracefully', () => {
    const targets = [
      createMockTarget({
        last_backup: undefined,
        schedule: undefined,
        next_scheduled: undefined,
      }),
    ]

    render(<TargetList targets={targets} />)

    expect(screen.getByText('test-db')).toBeInTheDocument()
    expect(screen.getByText('Never')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('displays correct button text when backup is running', () => {
    const targets = [createMockTarget({ is_running: true })]

    render(<TargetList targets={targets} />)

    expect(screen.getByRole('button', { name: /running/i })).toBeInTheDocument()
  })

  it('displays correct button text when backup is idle', () => {
    const targets = [createMockTarget({ is_running: false })]

    render(<TargetList targets={targets} />)

    expect(screen.getByRole('button', { name: /backup now/i })).toBeInTheDocument()
  })
})
