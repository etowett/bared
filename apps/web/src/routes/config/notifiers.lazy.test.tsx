/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen, waitFor } from '@/test/utils'
import type { ConfigSource, Notifier } from '@/types'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockToastError, mockToastSuccess } = vi.hoisted(() => ({
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}))

vi.mock('@/components/config/NotifierForm', () => ({
  NotifierForm: () => null,
}))

vi.mock('@/hooks/useConfig', () => ({
  useNotifiers: vi.fn(),
  useCreateNotifier: vi.fn(),
  useUpdateNotifier: vi.fn(),
  useDeleteNotifier: vi.fn(),
}))

import * as useConfigHook from '@/hooks/useConfig'
import { NotifiersPage } from './notifiers.lazy'

const notifier: Notifier = {
  name: 'ops-slack',
  type: 'slack',
  on_success: false,
  config: { url: 'https://hooks.slack.test/services/x' },
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const deleteMutation = { mutateAsync: vi.fn(), isPending: false }

function setup(source: ConfigSource = 'database') {
  vi.mocked(useConfigHook.useNotifiers).mockReturnValue({
    data: { notifiers: [notifier], source },
    isLoading: false,
    error: null,
  } as any)
  vi.mocked(useConfigHook.useDeleteNotifier).mockReturnValue(deleteMutation as any)
  vi.mocked(useConfigHook.useCreateNotifier).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)
  vi.mocked(useConfigHook.useUpdateNotifier).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
  } as any)

  return render(<NotifiersPage />)
}

describe('NotifiersPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    deleteMutation.isPending = false
    deleteMutation.mutateAsync.mockResolvedValue(undefined)
  })

  it('deletes the notifier once the confirmation is accepted', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete notifier ops-slack' }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete Notifier' }))

    await waitFor(() => expect(deleteMutation.mutateAsync).toHaveBeenCalledWith('ops-slack'))
  })

  it('does not delete when the confirmation is cancelled', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete notifier ops-slack' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('does not delete when the confirmation is dismissed with Escape', async () => {
    const user = userEvent.setup()
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete notifier ops-slack' }))
    await screen.findByRole('dialog')
    await user.keyboard('{Escape}')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleteMutation.mutateAsync).not.toHaveBeenCalled()
  })

  it('surfaces a delete failure instead of swallowing it', async () => {
    const user = userEvent.setup()
    deleteMutation.mutateAsync.mockRejectedValue(new Error('notifier not found'))
    setup()

    await user.click(screen.getByRole('button', { name: 'Delete notifier ops-slack' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Delete Notifier' }))

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith('Failed to delete notifier "ops-slack"', {
        description: 'notifier not found',
      })
    )
  })

  it('explains the read-only state and disables the actions for YAML configs', async () => {
    setup('yaml')

    expect(
      await screen.findByText(/These notifiers come from YAML and are read-only here/i)
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: /migrate this configuration to the database/i })
    ).toHaveAttribute('href', '/config')
    expect(screen.getByRole('button', { name: 'Edit notifier ops-slack' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete notifier ops-slack' })).toBeDisabled()
  })

  it('shows no read-only notice for database-sourced configs', () => {
    setup('database')

    expect(screen.queryByText(/read-only here/i)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit notifier ops-slack' })).toBeEnabled()
  })
})
