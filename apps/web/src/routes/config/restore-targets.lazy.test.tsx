/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen, waitFor } from '@/test/utils'
import type { ConfigSource, RestoreTargetConfig } from '@/types'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockToastError, mockToastSuccess } = vi.hoisted(() => ({
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}))

vi.mock('@/components/config/RestoreTargetForm', () => ({
  RestoreTargetForm: () => null,
}))

vi.mock('@/hooks/useConfig', () => ({
  useRestoreTargetsConfig: vi.fn(),
  useCreateRestoreTargetConfig: vi.fn(),
  useUpdateRestoreTargetConfig: vi.fn(),
  useDeleteRestoreTargetConfig: vi.fn(),
}))

import * as useConfigHook from '@/hooks/useConfig'
import { RestoreTargetsPage } from './restore-targets.lazy'

const restoreTarget: RestoreTargetConfig = {
  name: 'staging-restore',
  connection: {
    type: 'postgres',
    host: 'staging.internal',
    port: 5432,
    user: 'app',
    database: 'app',
  },
  storage_name: 'local-backup',
  source_target: 'prod-db',
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const deleteMutation = { mutateAsync: vi.fn(), isPending: false }

function setup(source: ConfigSource = 'database') {
  vi.mocked(useConfigHook.useRestoreTargetsConfig).mockReturnValue({
    data: { restore_targets: [restoreTarget], source },
    isLoading: false,
    error: null,
  } as any)
  vi.mocked(useConfigHook.useDeleteRestoreTargetConfig).mockReturnValue(deleteMutation as any)
  vi.mocked(useConfigHook.useCreateRestoreTargetConfig).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)
  vi.mocked(useConfigHook.useUpdateRestoreTargetConfig).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)

  return render(<RestoreTargetsPage />)
}

describe('RestoreTargetsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    deleteMutation.isPending = false
    deleteMutation.mutateAsync.mockResolvedValue(undefined)
  })

  it('deletes the restore target once the confirmation is accepted', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete restore target staging-restore' }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete Restore Target' }))

    await waitFor(() => expect(deleteMutation.mutateAsync).toHaveBeenCalledWith('staging-restore'))
  })

  it('does not delete when the confirmation is cancelled', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete restore target staging-restore' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('does not delete when the confirmation is dismissed with Escape', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete restore target staging-restore' }))
    await screen.findByRole('dialog')
    await user.keyboard('{Escape}')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('surfaces a delete failure instead of swallowing it', async () => {
    const user = userEvent.setup()
    deleteMutation.mutateAsync.mockRejectedValue(new Error('restore target is running'))
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete restore target staging-restore' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Delete Restore Target' }))

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        'Failed to delete restore target "staging-restore"',
        { description: 'restore target is running' }
      )
    )
  })

  it('explains the read-only state and disables the actions for YAML configs', async () => {
    setup('yaml')

    expect(
      await screen.findByText(/These restore targets come from YAML and are read-only here/i)
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: /migrate this configuration to the database/i })
    ).toHaveAttribute('href', '/config')
    expect(
      screen.getByRole('button', { name: 'Edit restore target staging-restore' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Delete restore target staging-restore' })
    ).toBeDisabled()
  })

  it('shows no read-only notice for database-sourced configs', () => {
    setup('database')

    expect(screen.queryByText(/read-only here/i)).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Edit restore target staging-restore' })
    ).toBeEnabled()
  })
})
