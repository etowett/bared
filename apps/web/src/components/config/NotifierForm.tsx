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
import type {
  Notifier,
  NotifierConfigRequest,
  NotifierRequest,
  NotifierType,
  WebhookAuthRequest,
  WebhookAuthType,
} from '../../types'

interface NotifierFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  notifier?: Notifier
  onSubmit: (notifier: NotifierRequest) => Promise<void>
}

type AuthTypeChoice = 'none' | WebhookAuthType

/**
 * Everything the form collects, flat. `config` is assembled per type on submit
 * so the payload only ever carries the keys the API actually reads.
 */
interface NotifierFormState {
  name: string
  type: NotifierType
  onSuccess: boolean
  // slack
  slackUrl: string
  channel: string
  // email
  smtpHost: string
  /** Kept as the raw input string; parsed to a number when the payload is built. */
  smtpPort: string
  smtpUsername: string
  smtpFrom: string
  /** Comma-separated in the input; sent as the `smtp_to` list. */
  smtpTo: string
  smtpUseTLS: boolean
  smtpPassword: string
  // webhook
  webhookUrl: string
  webhookMethod: string
  /** No editor yet, so existing headers are carried through an edit unchanged. */
  webhookHeaders?: Record<string, string>
  authType: AuthTypeChoice
  authUsername: string
  authHeaderName: string
  authPassword: string
  authToken: string
  authHeaderValue: string
}

const DEFAULT_SMTP_PORT = 587
const DEFAULT_WEBHOOK_METHOD = 'POST'

/**
 * Secrets come back from the API as `***REDACTED***`; they must never be
 * pre-filled into an input or they would be sent back as a literal password.
 *
 * The keys read here are the ones `notifierToResponse` emits — `url`,
 * `smtp_username`, `smtp_from`, `smtp_to`, `webhook_method`, `webhook_auth.*`.
 * Reading anything else means an edit silently resets that field.
 */
function initialState(notifier?: Notifier): NotifierFormState {
  const config = notifier?.config ?? {}
  const auth = (config.webhook_auth ?? {}) as Partial<WebhookAuthRequest>
  const smtpTo: string[] = Array.isArray(config.smtp_to) ? config.smtp_to : []

  return {
    name: notifier?.name ?? '',
    type: notifier?.type ?? 'slack',
    onSuccess: notifier?.on_success ?? false,
    slackUrl: config.url ?? '',
    channel: config.channel ?? '',
    smtpHost: config.smtp_host ?? '',
    smtpPort: String(config.smtp_port || DEFAULT_SMTP_PORT),
    smtpUsername: config.smtp_username ?? '',
    smtpFrom: config.smtp_from ?? '',
    smtpTo: smtpTo.join(', '),
    smtpUseTLS: config.smtp_use_tls ?? false,
    smtpPassword: '',
    webhookUrl: config.url ?? '',
    webhookMethod: config.webhook_method ?? DEFAULT_WEBHOOK_METHOD,
    webhookHeaders: config.webhook_headers,
    authType: auth.type ?? 'none',
    authUsername: auth.username ?? '',
    authHeaderName: auth.header_name ?? '',
    authPassword: '',
    authToken: '',
    authHeaderValue: '',
  }
}

function buildWebhookAuth(state: NotifierFormState): WebhookAuthRequest | undefined {
  if (state.authType === 'none') {
    return undefined
  }
  const auth: WebhookAuthRequest = { type: state.authType }
  if (state.authType === 'basic') {
    auth.username = state.authUsername
  }
  if (state.authType === 'header') {
    auth.header_name = state.authHeaderName
  }
  return auth
}

/**
 * Mirrors `requestToNotifier` in apps/api/internal/api/config_handlers.go. Any
 * key not listed there is dropped by the server, so the shapes must match.
 */
function buildConfig(state: NotifierFormState): NotifierConfigRequest {
  switch (state.type) {
    case 'slack':
      return {
        // The API reads `url` for both Slack and webhook notifiers.
        url: state.slackUrl,
        channel: state.channel,
      }
    case 'email':
      return {
        smtp_host: state.smtpHost,
        // The API decodes this as a JSON number (`.(float64)`) and drops a string.
        smtp_port: parseInt(state.smtpPort, 10) || DEFAULT_SMTP_PORT,
        smtp_username: state.smtpUsername,
        smtp_from: state.smtpFrom,
        smtp_to: state.smtpTo
          .split(',')
          .map((address) => address.trim())
          .filter(Boolean),
        smtp_use_tls: state.smtpUseTLS,
      }
    case 'webhook': {
      const auth = buildWebhookAuth(state)
      return {
        url: state.webhookUrl,
        webhook_method: state.webhookMethod,
        ...(state.webhookHeaders && Object.keys(state.webhookHeaders).length > 0
          ? { webhook_headers: state.webhookHeaders }
          : {}),
        ...(auth ? { webhook_auth: auth } : {}),
      }
    }
  }
}

function buildPayload(state: NotifierFormState): NotifierRequest {
  const payload: NotifierRequest = {
    name: state.name,
    type: state.type,
    on_success: state.onSuccess,
    config: buildConfig(state),
  }

  // Secrets travel top-level and only when the user typed one, so editing a
  // notifier without retyping them does not overwrite them with blanks.
  if (state.type === 'email' && state.smtpPassword) {
    payload.smtp_password = state.smtpPassword
  }
  if (state.type === 'webhook') {
    if (state.authType === 'basic' && state.authPassword) {
      payload.webhook_auth_password = state.authPassword
    }
    if (state.authType === 'bearer' && state.authToken) {
      payload.webhook_auth_token = state.authToken
    }
    if (state.authType === 'header' && state.authHeaderValue) {
      payload.webhook_auth_header_value = state.authHeaderValue
    }
  }

  return payload
}

export function NotifierForm({ open, onOpenChange, notifier, onSubmit }: NotifierFormProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        {/*
          The page keeps this dialog mounted and swaps `notifier` underneath it,
          so the fields live in a child that is mounted fresh on every open.
          Otherwise "Edit" would show whatever the previous session left behind.
        */}
        {open && (
          <NotifierFormFields
            key={notifier?.name ?? '__new__'}
            notifier={notifier}
            onOpenChange={onOpenChange}
            onSubmit={onSubmit}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function NotifierFormFields({ notifier, onOpenChange, onSubmit }: Omit<NotifierFormProps, 'open'>) {
  const isEdit = !!notifier

  const [formData, setFormData] = useState<NotifierFormState>(() => initialState(notifier))
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const update = <K extends keyof NotifierFormState>(key: K, value: NotifierFormState[K]) =>
    setFormData((prev) => ({ ...prev, [key]: value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)

    try {
      await onSubmit(buildPayload(formData))
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save notifier')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
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
            onChange={(e) => update('name', e.target.value)}
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
            onValueChange={(value) => update('type', value as NotifierType)}
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
            checked={formData.onSuccess}
            onCheckedChange={(checked) => update('onSuccess', checked === true)}
          />
          <Label htmlFor="on_success" className="cursor-pointer">
            Notify on success (default is failures only)
          </Label>
        </div>

        {formData.type === 'slack' && (
          <>
            <div className="space-y-2">
              <Label htmlFor="slack_url">
                Webhook URL <span className="text-red-500">*</span>
              </Label>
              <Input
                id="slack_url"
                type="url"
                value={formData.slackUrl}
                onChange={(e) => update('slackUrl', e.target.value)}
                placeholder="https://hooks.slack.com/services/..."
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="slack_channel">Channel (optional)</Label>
              <Input
                id="slack_channel"
                value={formData.channel}
                onChange={(e) => update('channel', e.target.value)}
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
                value={formData.smtpHost}
                onChange={(e) => update('smtpHost', e.target.value)}
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
                min="1"
                value={formData.smtpPort}
                onChange={(e) => update('smtpPort', e.target.value)}
                placeholder="587"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="smtp_username">
                SMTP Username <span className="text-red-500">*</span>
              </Label>
              <Input
                id="smtp_username"
                value={formData.smtpUsername}
                onChange={(e) => update('smtpUsername', e.target.value)}
                placeholder="user@example.com"
                required
              />
            </div>

            <PasswordInput
              label="SMTP Password"
              value={formData.smtpPassword}
              onChange={(value) => update('smtpPassword', value)}
              placeholder="••••••••"
              required={!isEdit}
              isEdit={isEdit}
            />

            <div className="flex items-center space-x-2">
              <Checkbox
                id="smtp_use_tls"
                checked={formData.smtpUseTLS}
                onCheckedChange={(checked) => update('smtpUseTLS', checked === true)}
              />
              <Label htmlFor="smtp_use_tls" className="cursor-pointer">
                Use implicit TLS (SMTPS, usually port 465)
              </Label>
            </div>

            <div className="space-y-2">
              <Label htmlFor="smtp_from">
                From Email <span className="text-red-500">*</span>
              </Label>
              <Input
                id="smtp_from"
                type="email"
                value={formData.smtpFrom}
                onChange={(e) => update('smtpFrom', e.target.value)}
                placeholder="backups@example.com"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="smtp_to">
                To Emails <span className="text-red-500">*</span>
              </Label>
              <Input
                id="smtp_to"
                value={formData.smtpTo}
                onChange={(e) => update('smtpTo', e.target.value)}
                placeholder="admin@example.com, oncall@example.com"
                required
              />
              <p className="text-xs text-gray-500">Comma-separated for multiple recipients</p>
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
                value={formData.webhookUrl}
                onChange={(e) => update('webhookUrl', e.target.value)}
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
                value={formData.webhookMethod}
                onChange={(e) => update('webhookMethod', e.target.value)}
                placeholder="POST"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="webhook_auth_type">Authentication</Label>
              <Select
                value={formData.authType}
                onValueChange={(value) => update('authType', value as AuthTypeChoice)}
              >
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

            {formData.authType === 'basic' && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="webhook_auth_username">
                    Username <span className="text-red-500">*</span>
                  </Label>
                  <Input
                    id="webhook_auth_username"
                    value={formData.authUsername}
                    onChange={(e) => update('authUsername', e.target.value)}
                    placeholder="backup-bot"
                    required
                  />
                </div>

                <PasswordInput
                  label="Password"
                  value={formData.authPassword}
                  onChange={(value) => update('authPassword', value)}
                  placeholder="••••••••"
                  required={!isEdit}
                  isEdit={isEdit}
                />
              </>
            )}

            {formData.authType === 'bearer' && (
              <PasswordInput
                label="Bearer Token"
                value={formData.authToken}
                onChange={(value) => update('authToken', value)}
                placeholder="••••••••"
                required={!isEdit}
                isEdit={isEdit}
              />
            )}

            {formData.authType === 'header' && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="webhook_auth_header_name">
                    Header Name <span className="text-red-500">*</span>
                  </Label>
                  <Input
                    id="webhook_auth_header_name"
                    value={formData.authHeaderName}
                    onChange={(e) => update('authHeaderName', e.target.value)}
                    placeholder="X-API-Key"
                    required
                  />
                </div>

                <PasswordInput
                  label="Header Value"
                  value={formData.authHeaderValue}
                  onChange={(value) => update('authHeaderValue', value)}
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
    </>
  )
}
