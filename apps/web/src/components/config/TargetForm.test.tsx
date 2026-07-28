import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../test/utils'
import type { TargetConfig, TargetConfigRequest } from '../../types'
import { TargetForm } from './TargetForm'

vi.mock('../../hooks/useConfig', () => ({
  useStorages: () => ({
    data: {
      storages: [
        { name: 'offsite', type: 'sftp' },
        { name: 'local-disk', type: 'local' },
      ],
    },
  }),
}))

const field = {
  name: () => screen.getByLabelText(/^name \*$/i),
  host: () => screen.getByLabelText(/^host \*$/i),
  port: () => screen.getByLabelText(/^port \*$/i),
  user: () => screen.getByLabelText(/^user \*$/i),
  password: () => screen.getByLabelText(/^password$/i),
  database: () => screen.getByLabelText(/^database \*$/i),
  excludeTables: () => screen.getByLabelText(/^exclude tables/i),
  additionalArgs: () => screen.getByLabelText(/^additional arguments/i),
  type: () => screen.getByRole('combobox', { name: /^database type \*$/i }),
  storage: () => screen.getByRole('combobox', { name: /^storage backend/i }),
  frequency: () => screen.getByRole('combobox', { name: /^frequency$/i }),
  compressEnabled: () => screen.getByRole('checkbox', { name: /enable compression/i }),
  advancedToggle: () => screen.getByRole('button', { name: /advanced options/i }),
}

const pgTarget: TargetConfig = {
  name: 'pg_prod',
  connection: {
    type: 'postgres',
    host: 'db.internal',
    port: 6543,
    user: 'svc',
    // The API filters the password out of responses; it comes back empty.
    password: '',
    database: 'appdb',
  },
  storage_name: 'offsite',
  schedule: '0 2 * * 0',
  compress: { enabled: true, type: 'tgz' },
  exclude_tables: ['audit_log', 'sessions'],
  additional_args: ['--no-owner', '--clean'],
  enabled: true,
  created_at: '',
  updated_at: '',
}

const mysqlTarget: TargetConfig = {
  name: 'mysql_stage',
  connection: {
    type: 'mysql',
    host: 'stage.internal',
    port: 3307,
    user: 'root',
    password: '',
    database: 'stagedb',
  },
  schedule: '0 2 * * *',
  enabled: true,
  created_at: '',
  updated_at: '',
}

function renderForm(target?: TargetConfig) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  const onOpenChange = vi.fn()
  const form = (open: boolean, current?: TargetConfig) => (
    <TargetForm open={open} onOpenChange={onOpenChange} target={current} onSubmit={onSubmit} />
  )

  // The page mounts this dialog once, closed and with no target, then only
  // toggles `open` — which is exactly what left a `useState` initializer in the
  // dialog reading `undefined` forever. Tests mount it the same way.
  const { rerender } = render(form(false, undefined))
  rerender(form(true, target))

  /** Closes the dialog and opens it again for `next` — what the page does. */
  const reopenWith = (next?: TargetConfig) => {
    rerender(form(false, next))
    rerender(form(true, next))
  }

  return { onSubmit, reopenWith, user: userEvent.setup() }
}

const submitted = (onSubmit: ReturnType<typeof vi.fn>): TargetConfigRequest =>
  onSubmit.mock.calls[0][0]

describe('TargetForm', () => {
  it('defaults a new target to the MySQL port', () => {
    renderForm()
    expect(field.port()).toHaveValue(3306)
  })

  // Regression: the port default used to be applied from a useEffect keyed on
  // `formData.type`, which also ran on mount and clobbered the saved port of an
  // existing target. Defaults now only apply when the user changes the type.
  it('preserves a non-default port when editing an existing target', () => {
    renderForm(pgTarget)
    expect(field.port()).toHaveValue(6543)
  })

  it('applies the engine default port when the type changes', async () => {
    const { user } = renderForm()

    expect(field.port()).toHaveValue(3306)

    await user.click(field.type())
    await user.click(screen.getByRole('option', { name: 'PostgreSQL' }))

    expect(field.port()).toHaveValue(5432)
  })

  // Regression for #126: the page mounts this dialog once and only toggles
  // `open`, so a `useState` initializer in the dialog itself ran while `target`
  // was still undefined and every Edit opened blank.
  it('pre-fills every field when opened for an existing target', () => {
    renderForm(pgTarget)

    expect(field.name()).toHaveValue('pg_prod')
    expect(field.host()).toHaveValue('db.internal')
    expect(field.port()).toHaveValue(6543)
    expect(field.user()).toHaveValue('svc')
    expect(field.database()).toHaveValue('appdb')
    expect(field.excludeTables()).toHaveValue('audit_log, sessions')
    expect(field.additionalArgs()).toHaveValue('--no-owner --clean')
    expect(field.type()).toHaveTextContent('PostgreSQL')
    expect(field.storage()).toHaveTextContent('offsite (sftp)')
    expect(field.frequency()).toHaveTextContent('Weekly')
    expect(field.compressEnabled()).toBeChecked()
    // Write-only by design: the API never echoes it back.
    expect(field.password()).toHaveValue('')
    expect(screen.getByText(/current value will be retained if left blank/i)).toBeInTheDocument()
  })

  it('shows the second target, not the first, when editing two in a row', () => {
    const { reopenWith } = renderForm(pgTarget)
    expect(field.host()).toHaveValue('db.internal')

    reopenWith(mysqlTarget)

    expect(field.name()).toHaveValue('mysql_stage')
    expect(field.host()).toHaveValue('stage.internal')
    expect(field.port()).toHaveValue(3307)
    expect(field.user()).toHaveValue('root')
    expect(field.database()).toHaveValue('stagedb')
    expect(field.type()).toHaveTextContent('MySQL')
    expect(field.storage()).toHaveTextContent('Use default storage')
    expect(field.frequency()).toHaveTextContent('Daily')
    expect(field.excludeTables()).toHaveValue('')
    expect(field.additionalArgs()).toHaveValue('')
    expect(field.compressEnabled()).not.toBeChecked()
  })

  it('opens a clean form when create follows an edit', async () => {
    const { reopenWith, user } = renderForm(pgTarget)

    reopenWith(undefined)

    expect(screen.getByRole('heading', { name: /create target/i })).toBeInTheDocument()
    expect(field.name()).toHaveValue('')
    expect(field.host()).toHaveValue('')
    expect(field.port()).toHaveValue(3306)
    expect(field.user()).toHaveValue('')
    expect(field.database()).toHaveValue('')
    expect(field.type()).toHaveTextContent('MySQL')
    expect(field.storage()).toHaveTextContent('Use default storage')

    // Creating starts with Advanced folded away — which also has to reset when
    // the previous open was an edit, where it starts expanded.
    expect(field.advancedToggle()).toHaveAttribute('aria-expanded', 'false')
    await user.click(field.advancedToggle())

    expect(field.excludeTables()).toHaveValue('')
    expect(field.additionalArgs()).toHaveValue('')
    expect(field.compressEnabled()).not.toBeChecked()
  })

  // The blank form was worse than cosmetic: submitting it PUT an empty host,
  // user and database over the target's stored connection.
  it('submits the stored connection and omits the password on an untouched edit', async () => {
    const { onSubmit, user } = renderForm(pgTarget)

    await user.click(screen.getByRole('button', { name: /^update$/i }))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    const payload = submitted(onSubmit)
    expect(payload).toMatchObject({
      name: 'pg_prod',
      connection: {
        type: 'postgres',
        host: 'db.internal',
        port: 6543,
        user: 'svc',
        database: 'appdb',
      },
      storage_name: 'offsite',
      schedule: '0 2 * * 0',
      compress: { enabled: true, type: 'tgz' },
      exclude_tables: ['audit_log', 'sessions'],
      additional_args: ['--no-owner', '--clean'],
    })
    // Blank means "keep the stored password", so it must not be sent at all.
    expect(payload.connection).not.toHaveProperty('password')
  })

  describe('sectioning', () => {
    it('groups the fields under named headings', () => {
      renderForm()

      for (const heading of ['Identity', 'Connection', 'Scheduling', 'Storage']) {
        expect(screen.getByRole('heading', { level: 3, name: heading })).toBeInTheDocument()
      }
    })

    it('folds the advanced options away for a new target', () => {
      renderForm()

      expect(field.advancedToggle()).toHaveAttribute('aria-expanded', 'false')
      // `hidden`, not a CSS class: collapsed fields leave the accessibility
      // tree, so a role query — which is what assistive tech sees — misses them.
      expect(screen.queryByRole('textbox', { name: /^exclude tables/i })).not.toBeInTheDocument()
      expect(
        screen.queryByRole('checkbox', { name: /enable compression/i })
      ).not.toBeInTheDocument()
    })

    it('opens the advanced options when editing, so Edit shows the whole target', () => {
      renderForm(pgTarget)

      expect(field.advancedToggle()).toHaveAttribute('aria-expanded', 'true')
      expect(field.excludeTables()).toHaveValue('audit_log, sessions')
    })

    it('keeps the buttons out of the scrolling region', () => {
      renderForm()

      const scroller = field.name().closest('.overflow-y-auto')
      expect(scroller).not.toBeNull()
      expect(scroller).not.toContainElement(screen.getByRole('button', { name: /^create$/i }))
    })
  })

  describe('validation', () => {
    it('reports every missing field instead of submitting', async () => {
      const { onSubmit, user } = renderForm()

      await user.click(screen.getByRole('button', { name: /^create$/i }))

      expect(onSubmit).not.toHaveBeenCalled()
      expect(await screen.findByText('A name is required.')).toBeInTheDocument()
      expect(screen.getByText('A host is required.')).toBeInTheDocument()
      expect(screen.getByText('A user is required.')).toBeInTheDocument()
      expect(screen.getByText('A database is required.')).toBeInTheDocument()
      expect(screen.getByText('A password is required.')).toBeInTheDocument()
    })

    it('points the field at its message and moves focus to the first one', async () => {
      const { user } = renderForm()

      await user.click(screen.getByRole('button', { name: /^create$/i }))

      expect(field.name()).toHaveFocus()
      expect(field.name()).toHaveAttribute('aria-invalid', 'true')
      expect(field.name()).toHaveAccessibleDescription('A name is required.')
    })

    it('rejects a port outside the valid range', async () => {
      const { onSubmit, user } = renderForm()

      await user.clear(field.port())
      await user.type(field.port(), '70000')
      await user.click(screen.getByRole('button', { name: /^create$/i }))

      expect(onSubmit).not.toHaveBeenCalled()
      expect(screen.getByText('Enter a port between 1 and 65535.')).toBeInTheDocument()
    })

    it('clears a message as soon as the field is corrected', async () => {
      const { user } = renderForm()

      await user.click(screen.getByRole('button', { name: /^create$/i }))
      expect(screen.getByText('A host is required.')).toBeInTheDocument()

      await user.type(field.host(), 'db.internal')

      expect(screen.queryByText('A host is required.')).not.toBeInTheDocument()
    })

    it('still accepts a blank password when editing', async () => {
      const { onSubmit, user } = renderForm(pgTarget)

      await user.click(screen.getByRole('button', { name: /^update$/i }))

      expect(onSubmit).toHaveBeenCalledTimes(1)
      expect(screen.queryByText('A password is required.')).not.toBeInTheDocument()
    })

    it('does not demand a user or database for redis', async () => {
      const { onSubmit, user } = renderForm()

      await user.type(field.name(), 'cache')
      await user.click(field.type())
      await user.click(screen.getByRole('option', { name: 'Redis' }))
      await user.type(field.host(), 'localhost')
      await user.click(screen.getByRole('button', { name: /^create$/i }))

      expect(onSubmit).toHaveBeenCalledTimes(1)
      expect(submitted(onSubmit).connection).toMatchObject({
        type: 'redis',
        host: 'localhost',
        port: 6379,
      })
    })
  })
})
