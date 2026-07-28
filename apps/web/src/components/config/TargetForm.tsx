import { useState } from 'react'
import { Button } from '../ui/button'
import { Dialog, DialogDescription, DialogTitle } from '../ui/dialog'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Checkbox } from '../ui/checkbox'
import { PasswordInput } from './PasswordInput'
import { CronBuilder } from './CronBuilder'
import {
  DisclosureSection,
  FieldError,
  FieldHint,
  FormDialogBody,
  FormDialogContent,
  FormDialogFooter,
  FormDialogHeader,
  FormError,
  FormSection,
} from './FormLayout'
import { useStorages } from '../../hooks/useConfig'
import type { ConnectionConfig, TargetConfig, TargetConfigRequest } from '../../types'

type DatabaseType = 'mysql' | 'postgres' | 'redis'

const DEFAULT_PORTS: Record<DatabaseType, number> = {
  mysql: 3306,
  postgres: 5432,
  redis: 6379,
}

interface TargetFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  target?: TargetConfig
  onSubmit: (target: TargetConfigRequest) => Promise<void>
}

/** Everything the form collects, flat. The payload is assembled on submit. */
interface TargetFormState {
  name: string
  type: DatabaseType
  host: string
  port: number
  user: string
  password: string
  database: string
  storage_name: string
  schedule: string
  compress_enabled: boolean
  compress_type: 'gzip' | 'tgz'
  exclude_tables: string
  additional_args: string
}

/** The fields validation can reject, in the order they appear in the form. */
const VALIDATED_FIELDS = ['name', 'host', 'port', 'user', 'password', 'database'] as const
type ValidatedField = (typeof VALIDATED_FIELDS)[number]
type FieldErrors = Partial<Record<ValidatedField, string>>

/**
 * Redis connections carry no `user`/`database`, so narrow on the discriminant
 * rather than casting the union away.
 */
function credentials(connection?: ConnectionConfig): { user: string; database: string } {
  if (!connection || connection.type === 'redis') {
    return { user: '', database: '' }
  }
  return { user: connection.user, database: connection.database }
}

/**
 * The password is deliberately left blank: the API filters it out of responses,
 * so there is nothing to pre-fill and a blank one means "keep the stored value".
 */
function initialState(target?: TargetConfig): TargetFormState {
  const connection = target?.connection
  const type: DatabaseType = connection?.type ?? 'mysql'
  const { user, database } = credentials(connection)

  return {
    name: target?.name ?? '',
    type,
    host: connection?.host ?? '',
    port: connection?.port ?? DEFAULT_PORTS[type],
    user,
    password: '',
    database,
    storage_name: target?.storage_name ?? '',
    schedule: target?.schedule ?? '0 2 * * *',
    compress_enabled: target?.compress?.enabled ?? false,
    compress_type: target?.compress?.type ?? 'gzip',
    exclude_tables: target?.exclude_tables?.join(', ') ?? '',
    additional_args: target?.additional_args?.join(' ') ?? '',
  }
}

/**
 * Checks the whole form at once and reports every problem.
 *
 * Field-at-a-time validation on blur would hide the second error until the
 * first is fixed; submitting is when the user has declared themselves done, so
 * that is when everything gets checked.
 */
function validate(data: TargetFormState, isEdit: boolean): FieldErrors {
  const errors: FieldErrors = {}

  if (!data.name.trim()) {
    errors.name = 'A name is required.'
  } else if (!isEdit && !/^[A-Za-z0-9._-]+$/.test(data.name)) {
    errors.name = 'Use only letters, numbers, dots, dashes and underscores.'
  }

  if (!data.host.trim()) errors.host = 'A host is required.'

  if (!Number.isInteger(data.port) || data.port < 1 || data.port > 65535) {
    errors.port = 'Enter a port between 1 and 65535.'
  }

  if (data.type !== 'redis') {
    if (!data.user.trim()) errors.user = 'A user is required.'
    if (!data.database.trim()) errors.database = 'A database is required.'
    // On edit, blank means "keep the stored password" — see `handleSubmit`.
    if (!isEdit && !data.password) errors.password = 'A password is required.'
  }

  return errors
}

export function TargetForm({ open, onOpenChange, target, onSubmit }: TargetFormProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <FormDialogContent className="max-w-3xl">
        {/*
          The page keeps this dialog mounted and swaps `target` underneath it, so
          the fields live in a child that is mounted fresh on every open. A
          `useState` initializer runs once per mount — without this, "Edit" would
          show whatever the component was born with, and saving would write those
          blanks over the target's connection.
        */}
        {open && (
          <TargetFormFields
            key={target?.name ?? '__new__'}
            target={target}
            onOpenChange={onOpenChange}
            onSubmit={onSubmit}
          />
        )}
      </FormDialogContent>
    </Dialog>
  )
}

function TargetFormFields({ target, onOpenChange, onSubmit }: Omit<TargetFormProps, 'open'>) {
  const isEdit = !!target
  const { data: storagesData } = useStorages()
  const storages = storagesData?.storages || []

  const [formData, setFormData] = useState<TargetFormState>(() => initialState(target))

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [errors, setErrors] = useState<FieldErrors>({})
  // Creating a target is the short path, so the advanced knobs stay folded.
  // Editing one is not: hiding half of a target's stored configuration behind a
  // toggle would make Edit misrepresent what the target actually does.
  const [showAdvanced, setShowAdvanced] = useState(isEdit)

  // Switching the database type resets the port to that engine's default.
  const handleTypeChange = (value: string) => {
    const type = value as DatabaseType
    setFormData((prev) => ({ ...prev, type, port: DEFAULT_PORTS[type] ?? prev.port }))
  }

  /** Clears a field's error as soon as the user starts fixing it. */
  const update = (patch: Partial<TargetFormState>) => {
    setFormData((prev) => ({ ...prev, ...patch }))
    const touched = Object.keys(patch) as ValidatedField[]
    setErrors((prev) => {
      if (!touched.some((field) => prev[field])) return prev
      const next = { ...prev }
      touched.forEach((field) => delete next[field])
      return next
    })
  }

  const describedBy = (field: ValidatedField, hintId?: string) =>
    errors[field] ? `${field}-error` : hintId

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const found = validate(formData, isEdit)
    if (Object.keys(found).length > 0) {
      setErrors(found)
      // Land the user on the first thing that is wrong, in reading order —
      // an error message they have to hunt for is barely an error message.
      // The ids are the same ones the labels point at, so there is nothing to
      // keep in sync with a second registry of refs.
      const first = VALIDATED_FIELDS.find((field) => found[field])
      const element = first && document.getElementById(first)
      if (element instanceof HTMLElement) element.focus()
      return
    }

    setErrors({})
    setIsSubmitting(true)

    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Dynamic connection object for union type
      const connection: any = {
        type: formData.type,
        host: formData.host,
        port: formData.port,
      }

      if (formData.type !== 'redis') {
        connection.user = formData.user
        connection.database = formData.database
        if (formData.password) {
          connection.password = formData.password
        }
      }

      const payload: TargetConfigRequest = {
        name: formData.name,
        connection,
        storage_name: formData.storage_name || undefined,
        schedule: formData.schedule || undefined,
      }

      if (formData.compress_enabled) {
        payload.compress = {
          enabled: true,
          type: formData.compress_type as 'gzip' | 'tgz',
        }
      }

      if (formData.exclude_tables) {
        payload.exclude_tables = formData.exclude_tables
          .split(',')
          .map((t) => t.trim())
          .filter((t) => t)
      }

      if (formData.additional_args) {
        payload.additional_args = formData.additional_args.split(' ').filter((a) => a)
      }

      await onSubmit(payload)
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save target')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <FormDialogHeader>
        <DialogTitle>{isEdit ? 'Edit Target' : 'Create Target'}</DialogTitle>
        <DialogDescription>
          {isEdit
            ? 'Update backup target configuration'
            : 'Configure a new backup target with database connection'}
        </DialogDescription>
      </FormDialogHeader>

      {/*
        `noValidate` hands validation to `validate()`. The `required` attributes
        stay — they are what tells assistive tech the field is mandatory — but
        the browser's own bubble would fire before `handleSubmit` runs and
        replace the inline messages with a tooltip that vanishes on scroll.
      */}
      <form onSubmit={handleSubmit} noValidate className="flex min-h-0 flex-1 flex-col">
        <FormDialogBody>
          {error && <FormError>{error}</FormError>}

          <FormSection title="Identity" description="What this target is called and what it runs.">
            <div className="space-y-2">
              <Label htmlFor="name">
                Name <span className="text-danger">*</span>
              </Label>
              <Input
                id="name"
                value={formData.name}
                onChange={(e) => update({ name: e.target.value })}
                placeholder="my-database"
                required
                disabled={isEdit}
                aria-invalid={errors.name ? true : undefined}
                aria-describedby={describedBy('name', isEdit ? 'name-hint' : undefined)}
              />
              {errors.name && <FieldError id="name-error">{errors.name}</FieldError>}
              {!errors.name && isEdit && (
                <FieldHint id="name-hint">Target name cannot be changed</FieldHint>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="type">
                Database Type <span className="text-danger">*</span>
              </Label>
              <Select value={formData.type} onValueChange={handleTypeChange} disabled={isEdit}>
                <SelectTrigger
                  id="type"
                  aria-describedby={isEdit ? 'type-hint' : undefined}
                  className="w-full"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="mysql">MySQL</SelectItem>
                  <SelectItem value="postgres">PostgreSQL</SelectItem>
                  <SelectItem value="redis">Redis</SelectItem>
                </SelectContent>
              </Select>
              {isEdit && <FieldHint id="type-hint">Database type cannot be changed</FieldHint>}
            </div>
          </FormSection>

          <FormSection title="Connection" description="Where the daemon reaches the database.">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="host">
                  Host <span className="text-danger">*</span>
                </Label>
                <Input
                  id="host"
                  value={formData.host}
                  onChange={(e) => update({ host: e.target.value })}
                  placeholder="localhost"
                  required
                  aria-invalid={errors.host ? true : undefined}
                  aria-describedby={describedBy('host')}
                />
                {errors.host && <FieldError id="host-error">{errors.host}</FieldError>}
              </div>

              <div className="space-y-2">
                <Label htmlFor="port">
                  Port <span className="text-danger">*</span>
                </Label>
                <Input
                  id="port"
                  type="number"
                  value={formData.port}
                  onChange={(e) => update({ port: parseInt(e.target.value) })}
                  required
                  aria-invalid={errors.port ? true : undefined}
                  aria-describedby={describedBy('port')}
                />
                {errors.port && <FieldError id="port-error">{errors.port}</FieldError>}
              </div>
            </div>

            {formData.type !== 'redis' && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="user">
                    User <span className="text-danger">*</span>
                  </Label>
                  <Input
                    id="user"
                    value={formData.user}
                    onChange={(e) => update({ user: e.target.value })}
                    placeholder="root"
                    required
                    aria-invalid={errors.user ? true : undefined}
                    aria-describedby={describedBy('user')}
                  />
                  {errors.user && <FieldError id="user-error">{errors.user}</FieldError>}
                </div>

                <PasswordInput
                  label="Password"
                  value={formData.password}
                  onChange={(value) => update({ password: value })}
                  placeholder="••••••••"
                  required={!isEdit}
                  isEdit={isEdit}
                  id="password"
                  error={errors.password}
                />

                <div className="space-y-2">
                  <Label htmlFor="database">
                    Database <span className="text-danger">*</span>
                  </Label>
                  <Input
                    id="database"
                    value={formData.database}
                    onChange={(e) => update({ database: e.target.value })}
                    placeholder="myapp"
                    required
                    aria-invalid={errors.database ? true : undefined}
                    aria-describedby={describedBy('database')}
                  />
                  {errors.database && (
                    <FieldError id="database-error">{errors.database}</FieldError>
                  )}
                </div>
              </>
            )}
          </FormSection>

          <FormSection title="Scheduling" description="When the daemon runs this backup by itself.">
            <CronBuilder
              label="Schedule (optional)"
              value={formData.schedule}
              onChange={(value) => update({ schedule: value })}
              required={false}
            />
          </FormSection>

          <FormSection title="Storage" description="Where the finished backup is written.">
            <div className="space-y-2">
              <Label htmlFor="storage_name">Storage Backend (optional)</Label>
              <Select
                value={formData.storage_name || '__default__'}
                onValueChange={(value) =>
                  update({ storage_name: value === '__default__' ? '' : value })
                }
              >
                <SelectTrigger id="storage_name" aria-describedby="storage-hint" className="w-full">
                  <SelectValue placeholder="Use default storage" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">Use default storage</SelectItem>
                  {storages.map((storage) => (
                    <SelectItem key={storage.name} value={storage.name}>
                      {storage.name} ({storage.type})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldHint id="storage-hint">
                Leave empty to use the default storage backend
              </FieldHint>
            </div>
          </FormSection>

          <DisclosureSection
            title="Advanced Options"
            description="Compression, excluded tables and raw dump arguments."
            open={showAdvanced}
            onOpenChange={setShowAdvanced}
          >
            <div className="flex items-center space-x-2">
              <Checkbox
                id="compress_enabled"
                checked={formData.compress_enabled}
                onCheckedChange={(checked) => update({ compress_enabled: checked as boolean })}
              />
              <Label htmlFor="compress_enabled" className="cursor-pointer">
                Enable compression
              </Label>
            </div>

            {formData.compress_enabled && (
              <div className="ml-6 space-y-2">
                <Label htmlFor="compress_type">Compression Type</Label>
                <Select
                  value={formData.compress_type}
                  onValueChange={(value) => update({ compress_type: value as 'gzip' | 'tgz' })}
                >
                  <SelectTrigger id="compress_type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="gzip">Gzip</SelectItem>
                    <SelectItem value="tgz">Tar + Gzip</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}

            {formData.type !== 'redis' && (
              <div className="space-y-2">
                <Label htmlFor="exclude_tables">Exclude Tables (optional)</Label>
                <Input
                  id="exclude_tables"
                  value={formData.exclude_tables}
                  onChange={(e) => update({ exclude_tables: e.target.value })}
                  placeholder="table1, table2"
                  aria-describedby="exclude-hint"
                />
                <FieldHint id="exclude-hint">Comma-separated list of tables to exclude</FieldHint>
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="additional_args">Additional Arguments (optional)</Label>
              <Input
                id="additional_args"
                value={formData.additional_args}
                onChange={(e) => update({ additional_args: e.target.value })}
                placeholder="--single-transaction --quick"
                aria-describedby="args-hint"
              />
              <FieldHint id="args-hint">
                Space-separated additional arguments for backup command
              </FieldHint>
            </div>
          </DisclosureSection>
        </FormDialogBody>

        <FormDialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </Button>
        </FormDialogFooter>
      </form>
    </>
  )
}
