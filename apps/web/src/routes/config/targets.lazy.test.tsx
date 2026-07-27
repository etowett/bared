/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen, waitFor } from '@/test/utils'
import type { ConfigSource, TargetConfig } from '@/types'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockToastError, mockToastSuccess } = vi.hoisted(() => ({
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}))

// The form owns its own dialog and API shape; this suite is about the list
// page's delete flow and its read-only state.
vi.mock('@/components/config/TargetForm', () => ({
  TargetForm: () => null,
}))

vi.mock('@/hooks/useConfig', () => ({
  useTargetsConfig: vi.fn(),
  useCreateTargetConfig: vi.fn(),
  useUpdateTargetConfig: vi.fn(),
  useDeleteTargetConfig: vi.fn(),
}))

import * as useConfigHook from '@/hooks/useConfig'
import { TargetsPage } from './targets.lazy'

const target: TargetConfig = {
  name: 'prod-db',
  connection: { type: 'mysql', host: 'db.internal', port: 3306, user: 'root', database: 'app' },
  storage_name: 'local-backup',
  schedule: '0 2 * * *',
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const deleteMutation = { mutateAsync: vi.fn(), isPending: false }

function setup(source: ConfigSource = 'database') {
  vi.mocked(useConfigHook.useTargetsConfig).mockReturnValue({
    data: { targets: [target], source },
    isLoading: false,
    error: null,
  } as any)
  vi.mocked(useConfigHook.useDeleteTargetConfig).mockReturnValue(deleteMutation as any)
  vi.mocked(useConfigHook.useCreateTargetConfig).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)
  vi.mocked(useConfigHook.useUpdateTargetConfig).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)

  return render(<TargetsPage />)
}

describe('TargetsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    deleteMutation.isPending = false
    deleteMutation.mutateAsync.mockResolvedValue(undefined)
  })

  it('deletes the target once the confirmation is accepted', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete target prod-db' }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete Target' }))

    await waitFor(() => expect(deleteMutation.mutateAsync).toHaveBeenCalledWith('prod-db'))
  })

  it('does not delete when the confirmation is cancelled', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete target prod-db' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('does not delete when the confirmation is dismissed with Escape', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete target prod-db' }))
    await screen.findByRole('dialog')
    await user.keyboard('{Escape}')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('surfaces a delete failure instead of swallowing it', async () => {
    const user = userEvent.setup()
    deleteMutation.mutateAsync.mockRejectedValue(new Error('target is in use'))
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete target prod-db' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Delete Target' }))

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith('Failed to delete target "prod-db"', {
        description: 'target is in use',
      })
    )
  })

  it('explains the read-only state and disables the actions for YAML configs', async () => {
    setup('yaml')

    expect(
      await screen.findByText(/These targets come from YAML and are read-only here/i)
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: /migrate this configuration to the database/i })
    ).toHaveAttribute('href', '/config')
    expect(screen.getByRole('link', { name: /import yaml into the database/i })).toHaveAttribute(
      'href',
      '/config/import'
    )
    expect(screen.getByRole('button', { name: 'Edit target prod-db' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete target prod-db' })).toBeDisabled()
  })

  it('shows no read-only notice for database-sourced configs', () => {
    setup('database')

    expect(screen.queryByText(/read-only here/i)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit target prod-db' })).toBeEnabled()
  })
})
