import { useState } from 'react'
import { Button } from '../ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/dialog'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Textarea } from '../ui/textarea'
import { PasswordInput } from './PasswordInput'
import { useStorages, useTargetsConfig } from '../../hooks/useConfig'
import type { ConnectionConfig, RestoreTargetConfig, RestoreTargetConfigRequest } from '../../types'

type DatabaseType = 'mysql' | 'postgres' | 'redis'

const DEFAULT_PORTS: Record<DatabaseType, number> = {
  mysql: 3306,
  postgres: 5432,
  redis: 6379,
}

interface RestoreTargetFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  target?: RestoreTargetConfig
  onSubmit: (target: RestoreTargetConfigRequest) => Promise<void>
}

/** Everything the form collects, flat. The payload is assembled on submit. */
interface RestoreTargetFormState {
  name: string
  type: DatabaseType
  host: string
  port: number
  user: string
  password: string
  database: string
  storage_name: string
  source_target: string
  description: string
}

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
function initialState(target?: RestoreTargetConfig): RestoreTargetFormState {
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
    source_target: target?.source_target ?? '',
    description: target?.description ?? '',
  }
}

export function RestoreTargetForm({
  open,
  onOpenChange,
  target,
  onSubmit,
}: RestoreTargetFormProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        {/*
          The page keeps this dialog mounted and swaps `target` underneath it, so
          the fields live in a child that is mounted fresh on every open. A
          `useState` initializer runs once per mount — without this, "Edit" would
          show whatever the component was born with, and saving would write those
          blanks over the restore target's connection.
        */}
        {open && (
          <RestoreTargetFormFields
            key={target?.name ?? '__new__'}
            target={target}
            onOpenChange={onOpenChange}
            onSubmit={onSubmit}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function RestoreTargetFormFields({
  target,
  onOpenChange,
  onSubmit,
}: Omit<RestoreTargetFormProps, 'open'>) {
  const isEdit = !!target
  const { data: storagesData } = useStorages()
  const { data: targetsData } = useTargetsConfig()
  const storages = storagesData?.storages || []
  const targets = targetsData?.targets || []

  const [formData, setFormData] = useState<RestoreTargetFormState>(() => initialState(target))

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Switching the database type resets the port to that engine's default.
  const handleTypeChange = (value: string) => {
    const type = value as DatabaseType
    setFormData((prev) => ({ ...prev, type, port: DEFAULT_PORTS[type] ?? prev.port }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
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

      const payload: RestoreTargetConfigRequest = {
        name: formData.name,
        connection,
        storage_name: formData.storage_name || undefined,
        source_target: formData.source_target || undefined,
        description: formData.description || undefined,
      }

      await onSubmit(payload)
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save restore target')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{isEdit ? 'Edit Restore Target' : 'Create Restore Target'}</DialogTitle>
        <DialogDescription>
          {isEdit
            ? 'Update restore target configuration'
            : 'Configure a new restore target for restoring backups'}
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 p-3 rounded-md text-sm">
            {error}
          </div>
        )}

        <div className="space-y-2">
          <Label htmlFor="name">
            Name <span className="text-red-500">*</span>
          </Label>
          <Input
            id="name"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            placeholder="my-restore-target"
            required
            disabled={isEdit}
          />
          {isEdit && <p className="text-xs text-gray-500">Restore target name cannot be changed</p>}
        </div>

        <div className="space-y-2">
          <Label htmlFor="description">Description (optional)</Label>
          <Textarea
            id="description"
            value={formData.description}
            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            placeholder="Development environment for testing restores"
            rows={2}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="source_target">Source Target (optional)</Label>
          <Select
            value={formData.source_target || '__none__'}
            onValueChange={(value) =>
              setFormData({ ...formData, source_target: value === '__none__' ? '' : value })
            }
          >
            <SelectTrigger id="source_target">
              <SelectValue placeholder="Select source target" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__none__">No specific source</SelectItem>
              {targets.map((t) => (
                <SelectItem key={t.name} value={t.name}>
                  {/* eslint-disable-next-line @typescript-eslint/no-explicit-any -- Union type access for display */}
                  {t.name} ({(t.connection as any).type})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-gray-500">
            Link to the backup target this restore target is associated with
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="type">
            Database Type <span className="text-red-500">*</span>
          </Label>
          <Select value={formData.type} onValueChange={handleTypeChange} disabled={isEdit}>
            <SelectTrigger id="type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="mysql">MySQL</SelectItem>
              <SelectItem value="postgres">PostgreSQL</SelectItem>
              <SelectItem value="redis">Redis</SelectItem>
            </SelectContent>
          </Select>
          {isEdit && <p className="text-xs text-gray-500">Database type cannot be changed</p>}
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label htmlFor="host">
              Host <span className="text-red-500">*</span>
            </Label>
            <Input
              id="host"
              value={formData.host}
              onChange={(e) => setFormData({ ...formData, host: e.target.value })}
              placeholder="localhost"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="port">
              Port <span className="text-red-500">*</span>
            </Label>
            <Input
              id="port"
              type="number"
              value={formData.port}
              onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
              required
            />
          </div>
        </div>

        {formData.type !== 'redis' && (
          <>
            <div className="space-y-2">
              <Label htmlFor="user">
                User <span className="text-red-500">*</span>
              </Label>
              <Input
                id="user"
                value={formData.user}
                onChange={(e) => setFormData({ ...formData, user: e.target.value })}
                placeholder="root"
                required
              />
            </div>

            <PasswordInput
              label="Password"
              value={formData.password}
              onChange={(value) => setFormData({ ...formData, password: value })}
              placeholder="••••••••"
              required={!isEdit}
              isEdit={isEdit}
            />

            <div className="space-y-2">
              <Label htmlFor="database">
                Database <span className="text-red-500">*</span>
              </Label>
              <Input
                id="database"
                value={formData.database}
                onChange={(e) => setFormData({ ...formData, database: e.target.value })}
                placeholder="myapp_restore"
                required
              />
            </div>
          </>
        )}

        <div className="space-y-2">
          <Label htmlFor="storage_name">Storage Backend (optional)</Label>
          <Select
            value={formData.storage_name || '__default__'}
            onValueChange={(value) =>
              setFormData({ ...formData, storage_name: value === '__default__' ? '' : value })
            }
          >
            <SelectTrigger id="storage_name">
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
          <p className="text-xs text-gray-500">Where to find backups for restoration</p>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </Button>
        </DialogFooter>
      </form>
    </>
  )
}
