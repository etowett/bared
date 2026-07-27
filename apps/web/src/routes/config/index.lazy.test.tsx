/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen, waitFor } from '@/test/utils'
import type { ConfigSource } from '@/types'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockToastError, mockToastSuccess } = vi.hoisted(() => ({
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}))

vi.mock('@/hooks/useConfig', () => ({
  useStorages: vi.fn(),
  useNotifiers: vi.fn(),
  useTargetsConfig: vi.fn(),
  useRestoreTargetsConfig: vi.fn(),
  useConfigSource: vi.fn(),
  useMigrateConfig: vi.fn(),
  useReloadConfig: vi.fn(),
}))

import * as useConfigHook from '@/hooks/useConfig'
import { ConfigDashboardPage } from './index.lazy'

const migrateMutation = { mutateAsync: vi.fn(), isPending: false }

function setup(source: ConfigSource = 'yaml') {
  const idle = { data: undefined, isLoading: false, error: null }
  vi.mocked(useConfigHook.useStorages).mockReturnValue({ ...idle, data: { storages: [] } } as any)
  vi.mocked(useConfigHook.useNotifiers).mockReturnValue({ ...idle, data: { notifiers: [] } } as any)
  vi.mocked(useConfigHook.useTargetsConfig).mockReturnValue({
    ...idle,
    data: { targets: [] },
  } as any)
  vi.mocked(useConfigHook.useRestoreTargetsConfig).mockReturnValue({
    ...idle,
    data: { restore_targets: [] },
  } as any)
  vi.mocked(useConfigHook.useConfigSource).mockReturnValue({ ...idle, data: { source } } as any)
  vi.mocked(useConfigHook.useMigrateConfig).mockReturnValue(migrateMutation as any)
  vi.mocked(useConfigHook.useReloadConfig).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)

  return render(<ConfigDashboardPage />)
}

describe('ConfigDashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    migrateMutation.isPending = false
    migrateMutation.mutateAsync.mockResolvedValue({
      storages_count: 1,
      notifiers_count: 2,
      targets_count: 3,
    })
  })

  it('migrates once the confirmation is accepted', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: /migrate to database/i }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => expect(migrateMutation.mutateAsync).toHaveBeenCalled())
    expect(await screen.findByText('Migration Successful')).toBeInTheDocument()
  })

  it('does not migrate when the confirmation is cancelled', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: /migrate to database/i }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(migrateMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('surfaces a migration failure instead of swallowing it', async () => {
    const user = userEvent.setup()
    migrateMutation.mutateAsync.mockRejectedValue(new Error('database is read-only'))
    setup()

    await user.click(screen.getByRole('button', { name: /migrate to database/i }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith('Migration failed', {
        description: 'database is read-only',
      })
    )
  })
})
