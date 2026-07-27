import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../test/utils'
import type { RestoreTargetConfig, RestoreTargetConfigRequest } from '../../types'
import { RestoreTargetForm } from './RestoreTargetForm'

vi.mock('../../hooks/useConfig', () => ({
  useStorages: () => ({
    data: {
      storages: [
        { name: 'offsite', type: 'sftp' },
        { name: 'local-disk', type: 'local' },
      ],
    },
  }),
  useTargetsConfig: () => ({
    data: {
      targets: [
        { name: 'pg_prod', connection: { type: 'postgres' } },
        { name: 'mysql_prod', connection: { type: 'mysql' } },
      ],
    },
  }),
}))

const field = {
  name: () => screen.getByLabelText(/^name \*$/i),
  description: () => screen.getByLabelText(/^description/i),
  host: () => screen.getByLabelText(/^host \*$/i),
  port: () => screen.getByLabelText(/^port \*$/i),
  user: () => screen.getByLabelText(/^user \*$/i),
  password: () => screen.getByLabelText(/^password$/i),
  database: () => screen.getByLabelText(/^database \*$/i),
  type: () => screen.getByRole('combobox', { name: /^database type \*$/i }),
  storage: () => screen.getByRole('combobox', { name: /^storage backend/i }),
  sourceTarget: () => screen.getByRole('combobox', { name: /^source target/i }),
}

const pgRestoreTarget: RestoreTargetConfig = {
  name: 'pg_staging',
  connection: {
    type: 'postgres',
    host: 'staging.internal',
    port: 6543,
    user: 'restorer',
    // The API filters the password out of responses; it comes back empty.
    password: '',
    database: 'appdb_restore',
  },
  storage_name: 'offsite',
  source_target: 'pg_prod',
  description: 'Staging clone of production',
  enabled: true,
  created_at: '',
  updated_at: '',
}

const mysqlRestoreTarget: RestoreTargetConfig = {
  name: 'mysql_scratch',
  connection: {
    type: 'mysql',
    host: 'scratch.internal',
    port: 3307,
    user: 'root',
    password: '',
    database: 'scratchdb',
  },
  enabled: true,
  created_at: '',
  updated_at: '',
}

function renderForm(target?: RestoreTargetConfig) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  const onOpenChange = vi.fn()
  const form = (open: boolean, current?: RestoreTargetConfig) => (
    <RestoreTargetForm
      open={open}
      onOpenChange={onOpenChange}
      target={current}
      onSubmit={onSubmit}
    />
  )

  // The page mounts this dialog once, closed and with no target, then only
  // toggles `open` — which is exactly what left a `useState` initializer in the
  // dialog reading `undefined` forever. Tests mount it the same way.
  const { rerender } = render(form(false, undefined))
  rerender(form(true, target))

  /** Closes the dialog and opens it again for `next` — what the page does. */
  const reopenWith = (next?: RestoreTargetConfig) => {
    rerender(form(false, next))
    rerender(form(true, next))
  }

  return { onSubmit, reopenWith, user: userEvent.setup() }
}

const submitted = (onSubmit: ReturnType<typeof vi.fn>): RestoreTargetConfigRequest =>
  onSubmit.mock.calls[0][0]

describe('RestoreTargetForm', () => {
  it('defaults a new restore target to the MySQL port', () => {
    renderForm()
    expect(field.port()).toHaveValue(3306)
  })

  it('applies the engine default port when the type changes', async () => {
    const { user } = renderForm()

    await user.click(field.type())
    await user.click(screen.getByRole('option', { name: 'PostgreSQL' }))

    expect(field.port()).toHaveValue(5432)
  })

  // Regression for #126: the page mounts this dialog once and only toggles
  // `open`, so a `useState` initializer in the dialog itself ran while `target`
  // was still undefined and every Edit opened blank.
  it('pre-fills every field when opened for an existing restore target', () => {
    renderForm(pgRestoreTarget)

    expect(field.name()).toHaveValue('pg_staging')
    expect(field.description()).toHaveValue('Staging clone of production')
    expect(field.host()).toHaveValue('staging.internal')
    expect(field.port()).toHaveValue(6543)
    expect(field.user()).toHaveValue('restorer')
    expect(field.database()).toHaveValue('appdb_restore')
    expect(field.type()).toHaveTextContent('PostgreSQL')
    expect(field.storage()).toHaveTextContent('offsite (sftp)')
    expect(field.sourceTarget()).toHaveTextContent('pg_prod')
    // Write-only by design: the API never echoes it back.
    expect(field.password()).toHaveValue('')
    expect(screen.getByText(/current value will be retained if left blank/i)).toBeInTheDocument()
  })

  it('shows the second restore target, not the first, when editing two in a row', () => {
    const { reopenWith } = renderForm(pgRestoreTarget)
    expect(field.host()).toHaveValue('staging.internal')

    reopenWith(mysqlRestoreTarget)

    expect(field.name()).toHaveValue('mysql_scratch')
    expect(field.description()).toHaveValue('')
    expect(field.host()).toHaveValue('scratch.internal')
    expect(field.port()).toHaveValue(3307)
    expect(field.user()).toHaveValue('root')
    expect(field.database()).toHaveValue('scratchdb')
    expect(field.type()).toHaveTextContent('MySQL')
    expect(field.storage()).toHaveTextContent('Use default storage')
    expect(field.sourceTarget()).toHaveTextContent('No specific source')
  })

  it('opens a clean form when create follows an edit', () => {
    const { reopenWith } = renderForm(pgRestoreTarget)

    reopenWith(undefined)

    expect(screen.getByRole('heading', { name: /create restore target/i })).toBeInTheDocument()
    expect(field.name()).toHaveValue('')
    expect(field.description()).toHaveValue('')
    expect(field.host()).toHaveValue('')
    expect(field.port()).toHaveValue(3306)
    expect(field.user()).toHaveValue('')
    expect(field.database()).toHaveValue('')
    expect(field.type()).toHaveTextContent('MySQL')
    expect(field.storage()).toHaveTextContent('Use default storage')
  })

  // The blank form was worse than cosmetic: submitting it PUT an empty host,
  // user and database over the restore target's stored connection.
  it('submits the stored connection and omits the password on an untouched edit', async () => {
    const { onSubmit, user } = renderForm(pgRestoreTarget)

    await user.click(screen.getByRole('button', { name: /^update$/i }))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    const payload = submitted(onSubmit)
    expect(payload).toMatchObject({
      name: 'pg_staging',
      connection: {
        type: 'postgres',
        host: 'staging.internal',
        port: 6543,
        user: 'restorer',
        database: 'appdb_restore',
      },
      storage_name: 'offsite',
      source_target: 'pg_prod',
      description: 'Staging clone of production',
    })
    // Blank means "keep the stored password", so it must not be sent at all.
    expect(payload.connection).not.toHaveProperty('password')
  })
})
