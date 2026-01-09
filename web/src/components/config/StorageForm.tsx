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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/select'
import { PasswordInput } from './PasswordInput'
import type { Storage, StorageRequest } from '../../types'

interface StorageFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  storage?: Storage
  onSubmit: (storage: StorageRequest) => Promise<void>
}

export function StorageForm({ open, onOpenChange, storage, onSubmit }: StorageFormProps) {
  const isEdit = !!storage

  const [formData, setFormData] = useState<StorageRequest>({
    name: storage?.name || '',
    type: storage?.type || 'local',
    keep: storage?.keep || 7,
    config: storage?.config || {},
    secret_access_key: '',
    password: '',
  })

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)

    try {
      // Build the config object based on storage type
      let config: Record<string, any> = {}

      if (formData.type === 'local') {
        config = {
          path: (document.getElementById('local_path') as HTMLInputElement)?.value || '',
        }
      } else if (formData.type === 's3') {
        config = {
          bucket: (document.getElementById('s3_bucket') as HTMLInputElement)?.value || '',
          region: (document.getElementById('s3_region') as HTMLInputElement)?.value || '',
          access_key_id: (document.getElementById('s3_access_key_id') as HTMLInputElement)?.value || '',
          endpoint: (document.getElementById('s3_endpoint') as HTMLInputElement)?.value || '',
        }
      } else if (formData.type === 'sftp') {
        config = {
          host: (document.getElementById('sftp_host') as HTMLInputElement)?.value || '',
          port: parseInt((document.getElementById('sftp_port') as HTMLInputElement)?.value || '22'),
          user: (document.getElementById('sftp_user') as HTMLInputElement)?.value || '',
          path: (document.getElementById('sftp_path') as HTMLInputElement)?.value || '',
        }
      }

      const payload: StorageRequest = {
        name: formData.name,
        type: formData.type,
        keep: formData.keep,
        config,
      }

      // Add secrets only if provided
      if (formData.type === 's3' && formData.secret_access_key) {
        payload.secret_access_key = formData.secret_access_key
      }
      if (formData.type === 'sftp' && formData.password) {
        payload.password = formData.password
      }

      await onSubmit(payload)
      onOpenChange(false)

      // Reset form
      setFormData({
        name: '',
        type: 'local',
        keep: 7,
        config: {},
        secret_access_key: '',
        password: '',
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save storage')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Storage' : 'Create Storage'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update storage backend configuration'
              : 'Configure a new storage backend for backups'}
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
              placeholder="my-storage"
              required
              disabled={isEdit}
            />
            {isEdit && (
              <p className="text-xs text-gray-500">Storage name cannot be changed</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="type">
              Type <span className="text-red-500">*</span>
            </Label>
            <Select
              value={formData.type}
              onValueChange={(value) => setFormData({ ...formData, type: value as any })}
              disabled={isEdit}
            >
              <SelectTrigger id="type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="local">Local</SelectItem>
                <SelectItem value="s3">S3</SelectItem>
                <SelectItem value="sftp">SFTP</SelectItem>
              </SelectContent>
            </Select>
            {isEdit && (
              <p className="text-xs text-gray-500">Storage type cannot be changed</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="keep">
              Retention (days) <span className="text-red-500">*</span>
            </Label>
            <Input
              id="keep"
              type="number"
              min="1"
              value={formData.keep}
              onChange={(e) => setFormData({ ...formData, keep: parseInt(e.target.value) })}
              required
            />
            <p className="text-xs text-gray-500">Number of days to keep backups</p>
          </div>

          {formData.type === 'local' && (
            <div className="space-y-2">
              <Label htmlFor="local_path">
                Path <span className="text-red-500">*</span>
              </Label>
              <Input
                id="local_path"
                defaultValue={storage?.config?.path || ''}
                placeholder="/var/backups"
                required
              />
            </div>
          )}

          {formData.type === 's3' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="s3_bucket">
                  Bucket <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="s3_bucket"
                  defaultValue={storage?.config?.bucket || ''}
                  placeholder="my-backup-bucket"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="s3_region">
                  Region <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="s3_region"
                  defaultValue={storage?.config?.region || ''}
                  placeholder="us-east-1"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="s3_access_key_id">
                  Access Key ID <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="s3_access_key_id"
                  defaultValue={storage?.config?.access_key_id || ''}
                  placeholder="AKIAIOSFODNN7EXAMPLE"
                  required
                />
              </div>

              <PasswordInput
                label="Secret Access Key"
                value={formData.secret_access_key || ''}
                onChange={(value) => setFormData({ ...formData, secret_access_key: value })}
                placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
                required={!isEdit}
                isEdit={isEdit}
              />

              <div className="space-y-2">
                <Label htmlFor="s3_endpoint">Endpoint (optional)</Label>
                <Input
                  id="s3_endpoint"
                  defaultValue={storage?.config?.endpoint || ''}
                  placeholder="https://s3.amazonaws.com"
                />
                <p className="text-xs text-gray-500">Leave blank for AWS S3</p>
              </div>
            </>
          )}

          {formData.type === 'sftp' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="sftp_host">
                  Host <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="sftp_host"
                  defaultValue={storage?.config?.host || ''}
                  placeholder="sftp.example.com"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="sftp_port">
                  Port <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="sftp_port"
                  type="number"
                  defaultValue={storage?.config?.port || 22}
                  placeholder="22"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="sftp_user">
                  User <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="sftp_user"
                  defaultValue={storage?.config?.user || ''}
                  placeholder="backup"
                  required
                />
              </div>

              <PasswordInput
                label="Password"
                value={formData.password || ''}
                onChange={(value) => setFormData({ ...formData, password: value })}
                placeholder="••••••••"
                required={!isEdit}
                isEdit={isEdit}
              />

              <div className="space-y-2">
                <Label htmlFor="sftp_path">
                  Remote Path <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="sftp_path"
                  defaultValue={storage?.config?.path || ''}
                  placeholder="/backups"
                  required
                />
              </div>
            </>
          )}

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
