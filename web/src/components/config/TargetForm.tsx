import { useState, useEffect } from 'react'
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
import { Checkbox } from '../ui/checkbox'
import { PasswordInput } from './PasswordInput'
import { CronBuilder } from './CronBuilder'
import { useStorages } from '../../hooks/useConfig'
import type { TargetConfig, TargetConfigRequest } from '../../types'

interface TargetFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  target?: TargetConfig
  onSubmit: (target: TargetConfigRequest) => Promise<void>
}

export function TargetForm({ open, onOpenChange, target, onSubmit }: TargetFormProps) {
  const isEdit = !!target
  const { data: storagesData } = useStorages()
  const storages = storagesData?.storages || []

  const [formData, setFormData] = useState({
    name: target?.name || '',
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Union type access in form initialization
    type: (target?.connection as any)?.type || 'mysql',
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Union type access in form initialization
    host: (target?.connection as any)?.host || '',
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Union type access in form initialization
    port: (target?.connection as any)?.port || 3306,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Union type access in form initialization
    user: (target?.connection as any)?.user || '',
    password: '',
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Union type access in form initialization
    database: (target?.connection as any)?.database || '',
    storage_name: target?.storage_name || '',
    schedule: target?.schedule || '0 2 * * *',
    compress_enabled: target?.compress?.enabled || false,
    compress_type: target?.compress?.type || 'gzip',
    exclude_tables: target?.exclude_tables?.join(', ') || '',
    additional_args: target?.additional_args?.join(' ') || '',
  })

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Update port when type changes
  useEffect(() => {
    if (formData.type === 'mysql') {
      setFormData((prev) => ({ ...prev, port: 3306 }))
    } else if (formData.type === 'postgres') {
      setFormData((prev) => ({ ...prev, port: 5432 }))
    } else if (formData.type === 'redis') {
      setFormData((prev) => ({ ...prev, port: 6379 }))
    }
  }, [formData.type])

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

      // Reset form
      setFormData({
        name: '',
        type: 'mysql',
        host: '',
        port: 3306,
        user: '',
        password: '',
        database: '',
        storage_name: '',
        schedule: '0 2 * * *',
        compress_enabled: false,
        compress_type: 'gzip',
        exclude_tables: '',
        additional_args: '',
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save target')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Target' : 'Create Target'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update backup target configuration'
              : 'Configure a new backup target with database connection'}
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
              placeholder="my-database"
              required
              disabled={isEdit}
            />
            {isEdit && <p className="text-xs text-gray-500">Target name cannot be changed</p>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="type">
              Database Type <span className="text-red-500">*</span>
            </Label>
            <Select
              value={formData.type}
              onValueChange={(value) =>
                setFormData({ ...formData, type: value as 'mysql' | 'postgres' | 'redis' })
              }
              disabled={isEdit}
            >
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
                  placeholder="myapp"
                  required
                />
              </div>
            </>
          )}

          <div className="space-y-2">
            <Label htmlFor="storage_name">Storage Backend (optional)</Label>
            <Select
              value={formData.storage_name || "__default__"}
              onValueChange={(value) => setFormData({ ...formData, storage_name: value === "__default__" ? "" : value })}
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
            <p className="text-xs text-gray-500">Leave empty to use the default storage backend</p>
          </div>

          <CronBuilder
            label="Schedule (optional)"
            value={formData.schedule}
            onChange={(value) => setFormData({ ...formData, schedule: value })}
            required={false}
          />

          <div className="space-y-4 border-t pt-4">
            <h3 className="font-medium">Advanced Options</h3>

            <div className="flex items-center space-x-2">
              <Checkbox
                id="compress_enabled"
                checked={formData.compress_enabled}
                onCheckedChange={(checked) =>
                  setFormData({ ...formData, compress_enabled: checked as boolean })
                }
              />
              <Label htmlFor="compress_enabled" className="cursor-pointer">
                Enable compression
              </Label>
            </div>

            {formData.compress_enabled && (
              <div className="space-y-2 ml-6">
                <Label htmlFor="compress_type">Compression Type</Label>
                <Select
                  value={formData.compress_type}
                  onValueChange={(value) =>
                    setFormData({ ...formData, compress_type: value as 'gzip' | 'tgz' })
                  }
                >
                  <SelectTrigger id="compress_type">
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
                  onChange={(e) => setFormData({ ...formData, exclude_tables: e.target.value })}
                  placeholder="table1, table2"
                />
                <p className="text-xs text-gray-500">Comma-separated list of tables to exclude</p>
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="additional_args">Additional Arguments (optional)</Label>
              <Input
                id="additional_args"
                value={formData.additional_args}
                onChange={(e) => setFormData({ ...formData, additional_args: e.target.value })}
                placeholder="--single-transaction --quick"
              />
              <p className="text-xs text-gray-500">
                Space-separated additional arguments for backup command
              </p>
            </div>
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
      </DialogContent>
    </Dialog>
  )
}
