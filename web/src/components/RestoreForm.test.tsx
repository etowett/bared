/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as useJobsHook from '../hooks/useJobs'
import * as useRestoreTargetsHook from '../hooks/useRestoreTargets'
import { render, screen, waitFor } from '../test/utils'
import { RestoreForm } from './RestoreForm'

// Create hoisted mocks
const { mockToastSuccess, mockToastError } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}))

// Mock sonner toast
vi.mock('sonner', () => ({
  toast: {
    success: mockToastSuccess,
    error: mockToastError,
  },
}))

// Mock hooks
vi.mock('../hooks/useJobs', () => ({
  useTriggerRestore: vi.fn(),
}))

vi.mock('../hooks/useRestoreTargets', () => ({
  useRestoreTargets: vi.fn(),
}))

describe('RestoreForm Component', () => {
  const mockTriggerRestore = {
    mutateAsync: vi.fn(),
    isPending: false,
  }

  const mockRestoreTargets = {
    restore_targets: [
      {
        name: 'restore-db1',
        type: 'postgres',
        database: 'test_db',
        host: 'localhost',
        description: 'Test restore target',
        source_target: 'backup-db1',
      },
      {
        name: 'restore-db2',
        type: 'mysql',
        database: 'prod_db',
        host: 'prod-server',
      },
    ],
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mockTriggerRestore.isPending = false
    vi.spyOn(useJobsHook, 'useTriggerRestore').mockReturnValue(mockTriggerRestore as any)
    vi.spyOn(useRestoreTargetsHook, 'useRestoreTargets').mockReturnValue({
      data: mockRestoreTargets,
      isLoading: false,
    } as any)
    // Clear localStorage
    window.localStorage.clear()
    ;(window.localStorage.getItem as any).mockReturnValue(null)
  })

  it('renders loading state while fetching restore targets', () => {
    vi.spyOn(useRestoreTargetsHook, 'useRestoreTargets').mockReturnValue({
      data: undefined,
      isLoading: true,
    } as any)

    render(<RestoreForm />)

    expect(screen.getByText(/loading restore targets/i)).toBeInTheDocument()
  })

  it('renders form with all required fields', () => {
    render(<RestoreForm />)

    expect(screen.getByLabelText(/restore target/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/backup path/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/dry-run/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /validate restore/i })).toBeInTheDocument()
  })

  it('populates restore target dropdown with targets', async () => {
    const user = userEvent.setup()
    render(<RestoreForm />)

    const select = screen.getByRole('combobox')
    await user.click(select)

    await waitFor(() => {
      expect(
        screen.getByRole('option', { name: /restore-db1.*postgres.*test_db.*localhost/i })
      ).toBeInTheDocument()
      expect(
        screen.getByRole('option', { name: /restore-db2.*mysql.*prod_db.*prod-server/i })
      ).toBeInTheDocument()
    })
  })

  it('shows target description when target is selected', async () => {
    const user = userEvent.setup()
    render(<RestoreForm />)

    const select = screen.getByRole('combobox')
    await user.click(select)

    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('option', { name: /restore-db1/i }))

    await waitFor(() => {
      expect(screen.getByText(/test restore target/i)).toBeInTheDocument()
      expect(screen.getByText(/backup-db1/i)).toBeInTheDocument()
    })
  })

  it('allows entering backup path', async () => {
    const user = userEvent.setup()
    render(<RestoreForm />)

    const input = screen.getByLabelText(/backup path/i)
    await user.type(input, '/backups/test.sql')

    expect(input).toHaveValue('/backups/test.sql')
  })

  it('validates required fields on submit', () => {
    render(<RestoreForm />)

    const submitButton = screen.getByRole('button', { name: /validate restore/i })

    // Button should be disabled when required fields are empty
    expect(submitButton).toBeDisabled()
    expect(mockTriggerRestore.mutateAsync).not.toHaveBeenCalled()
  })

  it('submits dry-run restore without confirmation', async () => {
    const user = userEvent.setup()
    mockTriggerRestore.mutateAsync.mockResolvedValueOnce({ job_id: 'test-job' })

    render(<RestoreForm />)

    // Select target
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))

    // Enter backup path
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')

    // Dry-run is checked by default, submit
    await user.click(screen.getByRole('button', { name: /validate restore/i }))

    await waitFor(() => {
      expect(mockTriggerRestore.mutateAsync).toHaveBeenCalledWith({
        target: 'restore-db1',
        backup_path: '/backups/test.sql',
        dry_run: true,
      })
      expect(mockToastSuccess).toHaveBeenCalledWith('Restore validation job queued successfully!')
    })
  })

  it('shows confirmation dialog for non-dry-run restore', async () => {
    const user = userEvent.setup()
    render(<RestoreForm />)

    // Select target
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))

    // Enter backup path
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')

    // Uncheck dry-run
    await user.click(screen.getByLabelText(/dry-run/i))

    // Submit
    await user.click(screen.getByRole('button', { name: /execute restore/i }))

    await waitFor(() => {
      expect(screen.getByText(/confirm restore/i)).toBeInTheDocument()
      expect(screen.getByText(/this will overwrite the existing database/i)).toBeInTheDocument()
    })

    expect(mockTriggerRestore.mutateAsync).not.toHaveBeenCalled()
  })

  it('executes restore after confirmation', async () => {
    const user = userEvent.setup()
    mockTriggerRestore.mutateAsync.mockResolvedValueOnce({ job_id: 'test-job' })

    render(<RestoreForm />)

    // Select target
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))

    // Enter backup path
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')

    // Uncheck dry-run
    await user.click(screen.getByLabelText(/dry-run/i))

    // Submit
    await user.click(screen.getByRole('button', { name: /execute restore/i }))

    // Confirm
    await waitFor(() => {
      expect(screen.getByText(/confirm restore/i)).toBeInTheDocument()
    })
    await user.click(screen.getByRole('button', { name: /yes, restore database/i }))

    await waitFor(() => {
      expect(mockTriggerRestore.mutateAsync).toHaveBeenCalledWith({
        target: 'restore-db1',
        backup_path: '/backups/test.sql',
        dry_run: false,
      })
      expect(mockToastSuccess).toHaveBeenCalledWith('Restore job queued successfully!')
    })
  })

  it('cancels restore from confirmation dialog', async () => {
    const user = userEvent.setup()
    render(<RestoreForm />)

    // Select target and path
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')

    // Uncheck dry-run and submit
    await user.click(screen.getByLabelText(/dry-run/i))
    await user.click(screen.getByRole('button', { name: /execute restore/i }))

    // Cancel
    await waitFor(() => {
      expect(screen.getByText(/confirm restore/i)).toBeInTheDocument()
    })
    await user.click(screen.getByRole('button', { name: /cancel/i }))

    await waitFor(() => {
      expect(screen.queryByText(/confirm restore/i)).not.toBeInTheDocument()
    })

    expect(mockTriggerRestore.mutateAsync).not.toHaveBeenCalled()
  })

  it('handles restore error', async () => {
    const user = userEvent.setup()
    mockTriggerRestore.mutateAsync.mockRejectedValueOnce(new Error('Network error'))

    render(<RestoreForm />)

    // Fill form
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')

    // Submit dry-run
    await user.click(screen.getByRole('button', { name: /validate restore/i }))

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith('Failed to trigger restore', {
        description: 'Network error',
      })
    })
  })

  it('resets form after successful restore', async () => {
    const user = userEvent.setup()
    mockTriggerRestore.mutateAsync.mockResolvedValueOnce({ job_id: 'test-job' })

    render(<RestoreForm />)

    // Fill form
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))

    const backupPathInput = screen.getByLabelText(/backup path/i)
    await user.type(backupPathInput, '/backups/test.sql')

    // Submit
    await user.click(screen.getByRole('button', { name: /validate restore/i }))

    await waitFor(() => {
      expect(mockTriggerRestore.mutateAsync).toHaveBeenCalled()
    })

    // Form should reset
    await waitFor(() => {
      expect(backupPathInput).toHaveValue('')
      expect(screen.getByLabelText(/dry-run/i)).toBeChecked()
    })
  })

  it('saves backup path to localStorage after successful restore', async () => {
    const user = userEvent.setup()
    mockTriggerRestore.mutateAsync.mockResolvedValueOnce({ job_id: 'test-job' })

    render(<RestoreForm />)

    // Fill and submit form
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))
    await user.type(screen.getByLabelText(/backup path/i), '/backups/new-path.sql')
    await user.click(screen.getByRole('button', { name: /validate restore/i }))

    await waitFor(() => {
      expect(window.localStorage.setItem).toHaveBeenCalledWith(
        'bared_backup_paths',
        JSON.stringify(['/backups/new-path.sql'])
      )
    })
  })

  it('loads and displays backup path suggestions from localStorage', async () => {
    const user = userEvent.setup()
    const mockHistory = ['/backups/path1.sql', '/backups/path2.sql', '/backups/path3.sql']
    ;(window.localStorage.getItem as any).mockReturnValue(JSON.stringify(mockHistory))

    render(<RestoreForm />)

    const input = screen.getByLabelText(/backup path/i)
    await user.type(input, 'path')

    await waitFor(() => {
      expect(screen.getByText('/backups/path1.sql')).toBeInTheDocument()
      expect(screen.getByText('/backups/path2.sql')).toBeInTheDocument()
      expect(screen.getByText('/backups/path3.sql')).toBeInTheDocument()
    })
  })

  it('filters suggestions based on input', async () => {
    const user = userEvent.setup()
    const mockHistory = ['/backups/mysql.sql', '/backups/postgres.sql', '/backups/mongo.sql']
    ;(window.localStorage.getItem as any).mockReturnValue(JSON.stringify(mockHistory))

    render(<RestoreForm />)

    const input = screen.getByLabelText(/backup path/i)
    await user.type(input, 'postgres')

    await waitFor(() => {
      expect(screen.getByText('/backups/postgres.sql')).toBeInTheDocument()
      expect(screen.queryByText('/backups/mysql.sql')).not.toBeInTheDocument()
      expect(screen.queryByText('/backups/mongo.sql')).not.toBeInTheDocument()
    })
  })

  it('selects suggestion on click', async () => {
    const user = userEvent.setup()
    const mockHistory = ['/backups/test.sql']
    ;(window.localStorage.getItem as any).mockReturnValue(JSON.stringify(mockHistory))

    render(<RestoreForm />)

    const input = screen.getByLabelText(/backup path/i)
    await user.type(input, 'test')

    await waitFor(() => {
      expect(screen.getByText('/backups/test.sql')).toBeInTheDocument()
    })

    await user.click(screen.getByText('/backups/test.sql'))

    expect(input).toHaveValue('/backups/test.sql')
    await waitFor(() => {
      expect(screen.queryByText('/backups/test.sql')).not.toBeInTheDocument() // Suggestions hidden
    })
  })

  it('handles keyboard navigation in suggestions (ArrowDown, ArrowUp)', async () => {
    const user = userEvent.setup()
    const mockHistory = ['/backups/path1.sql', '/backups/path2.sql', '/backups/path3.sql']
    ;(window.localStorage.getItem as any).mockReturnValue(JSON.stringify(mockHistory))

    render(<RestoreForm />)

    const input = screen.getByLabelText(/backup path/i)
    await user.type(input, 'path')

    await waitFor(() => {
      expect(screen.getByText('/backups/path1.sql')).toBeInTheDocument()
    })

    // Arrow down
    await user.keyboard('{ArrowDown}')
    await user.keyboard('{ArrowDown}')

    // Enter to select second item
    await user.keyboard('{Enter}')

    expect(input).toHaveValue('/backups/path2.sql')
  })

  it('handles Escape key to close suggestions', async () => {
    const user = userEvent.setup()
    const mockHistory = ['/backups/test.sql']
    ;(window.localStorage.getItem as any).mockReturnValue(JSON.stringify(mockHistory))

    render(<RestoreForm />)

    const input = screen.getByLabelText(/backup path/i)
    await user.type(input, 'test')

    await waitFor(() => {
      expect(screen.getByText('/backups/test.sql')).toBeInTheDocument()
    })

    await user.keyboard('{Escape}')

    await waitFor(() => {
      expect(screen.queryByText('/backups/test.sql')).not.toBeInTheDocument()
    })
  })

  it('disables submit button when form is incomplete', () => {
    render(<RestoreForm />)

    const submitButton = screen.getByRole('button', { name: /validate restore/i })
    expect(submitButton).toBeDisabled()
  })

  it('disables submit button when mutation is pending', async () => {
    const user = userEvent.setup()
    mockTriggerRestore.isPending = true

    render(<RestoreForm />)

    // Fill form
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')

    const submitButton = screen.getByRole('button', { name: /submitting/i })
    expect(submitButton).toBeDisabled()
  })

  it('calls onSuccess callback after successful restore', async () => {
    const user = userEvent.setup()
    const onSuccess = vi.fn()
    mockTriggerRestore.mutateAsync.mockResolvedValueOnce({ job_id: 'test-job' })

    render(<RestoreForm onSuccess={onSuccess} />)

    // Fill and submit form
    const select = screen.getByRole('combobox')
    await user.click(select)
    await waitFor(() => {
      expect(screen.getByRole('option', { name: /restore-db1/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('option', { name: /restore-db1/i }))
    await user.type(screen.getByLabelText(/backup path/i), '/backups/test.sql')
    await user.click(screen.getByRole('button', { name: /validate restore/i }))

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalled()
    })
  })

  it('changes button text and variant based on dry-run state', async () => {
    const user = userEvent.setup()
    render(<RestoreForm />)

    // Initially dry-run is checked
    expect(screen.getByRole('button', { name: /validate restore/i })).toBeInTheDocument()

    // Uncheck dry-run
    await user.click(screen.getByLabelText(/dry-run/i))

    expect(screen.getByRole('button', { name: /execute restore/i })).toBeInTheDocument()
  })
})
