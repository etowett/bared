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
import { Checkbox } from '../ui/checkbox'
import { PasswordInput } from './PasswordInput'
import type { Notifier, NotifierRequest } from '../../types'

interface NotifierFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  notifier?: Notifier
  onSubmit: (notifier: NotifierRequest) => Promise<void>
}

export function NotifierForm({ open, onOpenChange, notifier, onSubmit }: NotifierFormProps) {
  const isEdit = !!notifier

  const [formData, setFormData] = useState<NotifierRequest>({
    name: notifier?.name || '',
    type: notifier?.type || 'slack',
    on_success: notifier?.on_success ?? false,
    config: notifier?.config || {},
    smtp_password: '',
    webhook_auth_password: '',
    webhook_auth_token: '',
    webhook_auth_header_value: '',
  })

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)

    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Dynamic config object for union type
      let config: Record<string, any> = {}

      if (formData.type === 'slack') {
        config = {
          webhook_url:
            (document.getElementById('slack_webhook_url') as HTMLInputElement)?.value || '',
          channel: (document.getElementById('slack_channel') as HTMLInputElement)?.value || '',
        }
      } else if (formData.type === 'email') {
        config = {
          smtp_host: (document.getElementById('smtp_host') as HTMLInputElement)?.value || '',
          smtp_port: parseInt(
            (document.getElementById('smtp_port') as HTMLInputElement)?.value || '587'
          ),
          smtp_user: (document.getElementById('smtp_user') as HTMLInputElement)?.value || '',
          from_email: (document.getElementById('from_email') as HTMLInputElement)?.value || '',
          to_email: (document.getElementById('to_email') as HTMLInputElement)?.value || '',
        }
      } else if (formData.type === 'webhook') {
        config = {
          url: (document.getElementById('webhook_url') as HTMLInputElement)?.value || '',
          method: (document.getElementById('webhook_method') as HTMLInputElement)?.value || 'POST',
        }

        const authType = (document.getElementById('webhook_auth_type') as HTMLSelectElement)?.value
        if (authType && authType !== 'none') {
          config.auth_type = authType
        }
      }

      const payload: NotifierRequest = {
        name: formData.name,
        type: formData.type,
        on_success: formData.on_success,
        config,
      }

      // Add secrets only if provided
      if (formData.type === 'email' && formData.smtp_password) {
        payload.smtp_password = formData.smtp_password
      }
      if (formData.type === 'webhook') {
        if (config.auth_type === 'basic' && formData.webhook_auth_password) {
          payload.webhook_auth_password = formData.webhook_auth_password
        } else if (config.auth_type === 'bearer' && formData.webhook_auth_token) {
          payload.webhook_auth_token = formData.webhook_auth_token
        } else if (config.auth_type === 'header' && formData.webhook_auth_header_value) {
          payload.webhook_auth_header_value = formData.webhook_auth_header_value
          config.auth_header_name =
            (document.getElementById('webhook_auth_header_name') as HTMLInputElement)?.value || ''
        }
      }

      await onSubmit(payload)
      onOpenChange(false)

      // Reset form
      setFormData({
        name: '',
        type: 'slack',
        on_success: false,
        config: {},
        smtp_password: '',
        webhook_auth_password: '',
        webhook_auth_token: '',
        webhook_auth_header_value: '',
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save notifier')
    } finally {
      setIsSubmitting(false)
    }
  }

  const [webhookAuthType, setWebhookAuthType] = useState<string>(
    notifier?.config?.auth_type || 'none'
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Notifier' : 'Create Notifier'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update notification channel configuration'
              : 'Configure a new notification channel for backup alerts'}
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
              placeholder="my-notifier"
              required
              disabled={isEdit}
            />
            {isEdit && <p className="text-xs text-gray-500">Notifier name cannot be changed</p>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="type">
              Type <span className="text-red-500">*</span>
            </Label>
            <Select
              value={formData.type}
              onValueChange={(value) =>
                setFormData({ ...formData, type: value as 'slack' | 'email' | 'webhook' })
              }
              disabled={isEdit}
            >
              <SelectTrigger id="type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="slack">Slack</SelectItem>
                <SelectItem value="email">Email</SelectItem>
                <SelectItem value="webhook">Webhook</SelectItem>
              </SelectContent>
            </Select>
            {isEdit && <p className="text-xs text-gray-500">Notifier type cannot be changed</p>}
          </div>

          <div className="flex items-center space-x-2">
            <Checkbox
              id="on_success"
              checked={formData.on_success}
              onCheckedChange={(checked) =>
                setFormData({ ...formData, on_success: checked as boolean })
              }
            />
            <Label htmlFor="on_success" className="cursor-pointer">
              Notify on success (default is failures only)
            </Label>
          </div>

          {formData.type === 'slack' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="slack_webhook_url">
                  Webhook URL <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="slack_webhook_url"
                  type="url"
                  defaultValue={notifier?.config?.webhook_url || ''}
                  placeholder="https://hooks.slack.com/services/..."
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="slack_channel">Channel (optional)</Label>
                <Input
                  id="slack_channel"
                  defaultValue={notifier?.config?.channel || ''}
                  placeholder="#backups"
                />
              </div>
            </>
          )}

          {formData.type === 'email' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="smtp_host">
                  SMTP Host <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="smtp_host"
                  defaultValue={notifier?.config?.smtp_host || ''}
                  placeholder="smtp.gmail.com"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="smtp_port">
                  SMTP Port <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="smtp_port"
                  type="number"
                  defaultValue={notifier?.config?.smtp_port || 587}
                  placeholder="587"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="smtp_user">
                  SMTP User <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="smtp_user"
                  defaultValue={notifier?.config?.smtp_user || ''}
                  placeholder="user@example.com"
                  required
                />
              </div>

              <PasswordInput
                label="SMTP Password"
                value={formData.smtp_password || ''}
                onChange={(value) => setFormData({ ...formData, smtp_password: value })}
                placeholder="••••••••"
                required={!isEdit}
                isEdit={isEdit}
              />

              <div className="space-y-2">
                <Label htmlFor="from_email">
                  From Email <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="from_email"
                  type="email"
                  defaultValue={notifier?.config?.from_email || ''}
                  placeholder="backups@example.com"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="to_email">
                  To Email <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="to_email"
                  type="email"
                  defaultValue={notifier?.config?.to_email || ''}
                  placeholder="admin@example.com"
                  required
                />
              </div>
            </>
          )}

          {formData.type === 'webhook' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="webhook_url">
                  URL <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="webhook_url"
                  type="url"
                  defaultValue={notifier?.config?.url || ''}
                  placeholder="https://api.example.com/webhook"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="webhook_method">
                  Method <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="webhook_method"
                  defaultValue={notifier?.config?.method || 'POST'}
                  placeholder="POST"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="webhook_auth_type">Authentication</Label>
                <Select value={webhookAuthType} onValueChange={setWebhookAuthType}>
                  <SelectTrigger id="webhook_auth_type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None</SelectItem>
                    <SelectItem value="basic">Basic Auth</SelectItem>
                    <SelectItem value="bearer">Bearer Token</SelectItem>
                    <SelectItem value="header">Custom Header</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {webhookAuthType === 'basic' && (
                <PasswordInput
                  label="Password"
                  value={formData.webhook_auth_password || ''}
                  onChange={(value) => setFormData({ ...formData, webhook_auth_password: value })}
                  placeholder="••••••••"
                  required={!isEdit}
                  isEdit={isEdit}
                />
              )}

              {webhookAuthType === 'bearer' && (
                <PasswordInput
                  label="Bearer Token"
                  value={formData.webhook_auth_token || ''}
                  onChange={(value) => setFormData({ ...formData, webhook_auth_token: value })}
                  placeholder="••••••••"
                  required={!isEdit}
                  isEdit={isEdit}
                />
              )}

              {webhookAuthType === 'header' && (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="webhook_auth_header_name">
                      Header Name <span className="text-red-500">*</span>
                    </Label>
                    <Input
                      id="webhook_auth_header_name"
                      defaultValue={notifier?.config?.auth_header_name || ''}
                      placeholder="X-API-Key"
                      required
                    />
                  </div>

                  <PasswordInput
                    label="Header Value"
                    value={formData.webhook_auth_header_value || ''}
                    onChange={(value) =>
                      setFormData({ ...formData, webhook_auth_header_value: value })
                    }
                    placeholder="••••••••"
                    required={!isEdit}
                    isEdit={isEdit}
                  />
                </>
              )}
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
