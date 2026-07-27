/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen, waitFor } from '@/test/utils'
import type { ConfigSource, Storage } from '@/types'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockToastError, mockToastSuccess } = vi.hoisted(() => ({
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}))

vi.mock('@/components/config/StorageForm', () => ({
  StorageForm: () => null,
}))

vi.mock('@/hooks/useConfig', () => ({
  useStorages: vi.fn(),
  useCreateStorage: vi.fn(),
  useUpdateStorage: vi.fn(),
  useDeleteStorage: vi.fn(),
}))

import * as useConfigHook from '@/hooks/useConfig'
import { StoragesPage } from './storages.lazy'

const storage: Storage = {
  name: 'local-backup',
  type: 'local',
  keep: 5,
  config: { path: '/backups' },
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const deleteMutation = { mutateAsync: vi.fn(), isPending: false }

function setup(source: ConfigSource = 'database') {
  vi.mocked(useConfigHook.useStorages).mockReturnValue({
    data: { storages: [storage], source },
    isLoading: false,
    error: null,
  } as any)
  vi.mocked(useConfigHook.useDeleteStorage).mockReturnValue(deleteMutation as any)
  vi.mocked(useConfigHook.useCreateStorage).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)
  vi.mocked(useConfigHook.useUpdateStorage).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)

  return render(<StoragesPage />)
}

describe('StoragesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    deleteMutation.isPending = false
    deleteMutation.mutateAsync.mockResolvedValue(undefined)
  })

  it('deletes the storage once the confirmation is accepted', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete storage backend local-backup' }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete Storage' }))

    await waitFor(() => expect(deleteMutation.mutateAsync).toHaveBeenCalledWith('local-backup'))
  })

  it('does not delete when the confirmation is cancelled', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete storage backend local-backup' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('does not delete when the confirmation is dismissed with Escape', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete storage backend local-backup' }))
    await screen.findByRole('dialog')
    await user.keyboard('{Escape}')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('surfaces a delete failure instead of swallowing it', async () => {
    const user = userEvent.setup()
    deleteMutation.mutateAsync.mockRejectedValue(new Error('storage still referenced'))
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete storage backend local-backup' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Delete Storage' }))

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith('Failed to delete storage "local-backup"', {
        description: 'storage still referenced',
      })
    )
  })

  it('explains the read-only state and disables the actions for YAML configs', async () => {
    setup('yaml')

    expect(
      await screen.findByText(/These storage backends come from YAML and are read-only here/i)
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: /migrate this configuration to the database/i })
    ).toHaveAttribute('href', '/config')
    expect(screen.getByRole('button', { name: 'Edit storage backend local-backup' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Delete storage backend local-backup' })
    ).toBeDisabled()
  })

  it('shows no read-only notice for database-sourced configs', () => {
    setup('database')

    expect(screen.queryByText(/read-only here/i)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit storage backend local-backup' })).toBeEnabled()
  })
})
