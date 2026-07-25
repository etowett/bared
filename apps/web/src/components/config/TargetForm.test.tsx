/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../test/utils'
import type { TargetConfig } from '../../types'
import { TargetForm } from './TargetForm'

vi.mock('../../hooks/useConfig', () => ({
  useStorages: () => ({ data: { storages: [] } }),
}))

function renderForm(target?: TargetConfig) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  render(<TargetForm open onOpenChange={vi.fn()} target={target} onSubmit={onSubmit} />)
  return { onSubmit }
}

const portInput = () => screen.getByLabelText(/port/i) as HTMLInputElement

describe('TargetForm', () => {
  it('defaults a new target to the MySQL port', () => {
    renderForm()
    expect(portInput().value).toBe('3306')
  })

  // Regression: the port default used to be applied from a useEffect keyed on
  // `formData.type`, which also ran on mount and clobbered the saved port of an
  // existing target. Defaults now only apply when the user changes the type.
  it('preserves a non-default port when editing an existing target', () => {
    renderForm({
      name: 'pg_prod',
      connection: { type: 'postgres', host: 'db.internal', port: 6543, user: 'svc' },
    } as any)

    expect(portInput().value).toBe('6543')
  })

  it('applies the engine default port when the type changes', async () => {
    const user = userEvent.setup()
    renderForm()

    expect(portInput().value).toBe('3306')

    await user.click(screen.getByRole('combobox', { name: /database type/i }))
    await user.click(screen.getByRole('option', { name: 'PostgreSQL' }))

    expect(portInput().value).toBe('5432')
  })
})
