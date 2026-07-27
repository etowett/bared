import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../test/utils'
import type { Notifier, NotifierRequest, NotifierType } from '../../types'
import { NotifierForm } from './NotifierForm'

/**
 * The config keys `requestToNotifier` (apps/api/internal/api/config_handlers.go)
 * actually reads, per type. Anything else the form sends is silently dropped by
 * the server — which is how Slack shipped sending `webhook_url` while the API
 * read `url`, so the notifier saved, showed as configured, and never fired.
 */
const BACKEND_CONFIG_KEYS: Record<NotifierType, string[]> = {
  slack: ['url', 'channel'],
  email: ['smtp_host', 'smtp_port', 'smtp_username', 'smtp_from', 'smtp_to', 'smtp_use_tls'],
  webhook: ['url', 'webhook_method'],
}

/** Read by the API too, but only sent when the user configured them. */
const ALLOWED_EXTRA_CONFIG_KEYS: Record<NotifierType, string[]> = {
  slack: [],
  email: [],
  webhook: ['webhook_auth', 'webhook_headers'],
}

type User = ReturnType<typeof userEvent.setup>

const field = {
  name: () => screen.getByLabelText(/^name \*$/i),
  slackUrl: () => screen.getByLabelText(/^webhook url \*$/i),
  channel: () => screen.getByLabelText(/^channel/i),
  smtpHost: () => screen.getByLabelText(/^smtp host \*$/i),
  smtpPort: () => screen.getByLabelText(/^smtp port \*$/i),
  smtpUsername: () => screen.getByLabelText(/^smtp username \*$/i),
  smtpPassword: () => screen.getByLabelText(/^smtp password/i),
  smtpUseTLS: () => screen.getByRole('checkbox', { name: /implicit tls/i }),
  smtpFrom: () => screen.getByLabelText(/^from email \*$/i),
  smtpTo: () => screen.getByLabelText(/^to emails \*$/i),
  webhookUrl: () => screen.getByLabelText(/^url \*$/i),
  webhookMethod: () => screen.getByLabelText(/^method \*$/i),
  authUsername: () => screen.getByLabelText(/^username \*$/i),
  authPassword: () => screen.getByLabelText(/^password/i),
  authToken: () => screen.getByLabelText(/^bearer token/i),
  authHeaderName: () => screen.getByLabelText(/^header name \*$/i),
  authHeaderValue: () => screen.getByLabelText(/^header value/i),
}

function renderForm(notifier?: Notifier) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  render(<NotifierForm open onOpenChange={vi.fn()} notifier={notifier} onSubmit={onSubmit} />)
  return { onSubmit, user: userEvent.setup() }
}

async function selectOption(user: User, comboboxName: RegExp, optionLabel: string) {
  await user.click(screen.getByRole('combobox', { name: comboboxName }))
  await user.click(screen.getByRole('option', { name: optionLabel }))
}

const selectType = (user: User, optionLabel: string) =>
  selectOption(user, /^type \*$/i, optionLabel)

const selectAuth = (user: User, optionLabel: string) =>
  selectOption(user, /^authentication$/i, optionLabel)

async function submit(user: User) {
  await user.click(screen.getByRole('button', { name: /^create$/i }))
}

const submitted = (onSubmit: ReturnType<typeof vi.fn>): NotifierRequest => onSubmit.mock.calls[0][0]

async function fillSlack(user: User) {
  await user.type(field.name(), 'alerts')
  await user.type(field.slackUrl(), 'https://hooks.slack.com/services/T/B/x')
}

async function fillEmail(user: User) {
  await user.type(field.name(), 'ops-mail')
  await selectType(user, 'Email')
  await user.type(field.smtpHost(), 'smtp.example.com')
  await user.clear(field.smtpPort())
  await user.type(field.smtpPort(), '465')
  await user.type(field.smtpUsername(), 'mailer@example.com')
  await user.type(field.smtpFrom(), 'backups@example.com')
  await user.type(field.smtpTo(), 'admin@example.com, oncall@example.com')
  // Required on create, so every email flow has to fill it in.
  await user.type(field.smtpPassword(), 'hunter2')
}

const existingEmailNotifier: Notifier = {
  name: 'ops-mail',
  type: 'email',
  on_success: true,
  config: {
    smtp_host: 'smtp.example.com',
    smtp_port: 465,
    smtp_username: 'mailer@example.com',
    smtp_from: 'backups@example.com',
    smtp_to: ['admin@example.com', 'oncall@example.com'],
    smtp_use_tls: true,
    smtp_password: '***REDACTED***',
  },
  enabled: true,
  created_at: '',
  updated_at: '',
}

async function fillWebhook(user: User) {
  await user.type(field.name(), 'ops-hook')
  await selectType(user, 'Webhook')
  await user.type(field.webhookUrl(), 'https://api.example.com/webhook')
}

describe('NotifierForm', () => {
  // Regression for #105: the form sent `webhook_url` while the API reads `url`,
  // so every Slack notifier created from the dashboard was stored with an empty
  // URL — it saved, showed as configured, and then never fired.
  it('sends a Slack payload matching the backend NotifierRequest contract', async () => {
    const { onSubmit, user } = renderForm()

    await fillSlack(user)
    await user.type(field.channel(), '#backups')

    await submit(user)

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(submitted(onSubmit)).toEqual({
      name: 'alerts',
      type: 'slack',
      on_success: false,
      config: {
        // `url`, not `webhook_url` — the handler ignores `webhook_url`.
        url: 'https://hooks.slack.com/services/T/B/x',
        channel: '#backups',
      },
    })
  })

  // Same class of drift: the email branch sent `smtp_user`, `from_email` and a
  // single `to_email` string, none of which the API reads.
  it('sends an email payload matching the backend NotifierRequest contract', async () => {
    const { onSubmit, user } = renderForm()

    await fillEmail(user)
    await user.click(field.smtpUseTLS())

    await submit(user)

    expect(submitted(onSubmit)).toEqual({
      name: 'ops-mail',
      type: 'email',
      on_success: false,
      config: {
        smtp_host: 'smtp.example.com',
        // A JSON number: the handler reads `req.Config["smtp_port"].(float64)`.
        smtp_port: 465,
        smtp_username: 'mailer@example.com',
        smtp_from: 'backups@example.com',
        // A list, not a single address string.
        smtp_to: ['admin@example.com', 'oncall@example.com'],
        smtp_use_tls: true,
      },
      // Secrets ride top-level so the API never echoes them back in a response.
      smtp_password: 'hunter2',
    })
  })

  // The webhook branch sent `method` (API reads `webhook_method`) and a flat
  // `auth_type`/`auth_header_name`, while the API only builds a WebhookAuth from
  // a nested `webhook_auth` object — and only then applies the auth secrets.
  it('nests webhook auth under webhook_auth with the username the API reads', async () => {
    const { onSubmit, user } = renderForm()

    await fillWebhook(user)
    await user.clear(field.webhookMethod())
    await user.type(field.webhookMethod(), 'PUT')
    await selectAuth(user, 'Basic Auth')
    await user.type(field.authUsername(), 'backup-bot')
    await user.type(field.authPassword(), 's3cret')

    await submit(user)

    expect(submitted(onSubmit)).toEqual({
      name: 'ops-hook',
      type: 'webhook',
      on_success: false,
      config: {
        url: 'https://api.example.com/webhook',
        webhook_method: 'PUT',
        webhook_auth: { type: 'basic', username: 'backup-bot' },
      },
      webhook_auth_password: 's3cret',
    })
  })

  it('sends the custom header name inside webhook_auth', async () => {
    const { onSubmit, user } = renderForm()

    await fillWebhook(user)
    await selectAuth(user, 'Custom Header')
    await user.type(field.authHeaderName(), 'X-API-Key')
    await user.type(field.authHeaderValue(), 'abc123')

    await submit(user)

    const payload = submitted(onSubmit)
    expect(payload.config).toEqual({
      url: 'https://api.example.com/webhook',
      webhook_method: 'POST',
      webhook_auth: { type: 'header', header_name: 'X-API-Key' },
    })
    expect(payload.webhook_auth_header_value).toBe('abc123')
  })

  it('sends a bearer token top-level with only the type nested', async () => {
    const { onSubmit, user } = renderForm()

    await fillWebhook(user)
    await selectAuth(user, 'Bearer Token')
    await user.type(field.authToken(), 'tok_123')

    await submit(user)

    const payload = submitted(onSubmit)
    expect(payload.config).toEqual({
      url: 'https://api.example.com/webhook',
      webhook_method: 'POST',
      webhook_auth: { type: 'bearer' },
    })
    expect(payload.webhook_auth_token).toBe('tok_123')
  })

  it('omits webhook_auth entirely when authentication is None', async () => {
    const { onSubmit, user } = renderForm()

    await fillWebhook(user)
    await submit(user)

    expect(submitted(onSubmit).config).toEqual({
      url: 'https://api.example.com/webhook',
      webhook_method: 'POST',
    })
  })

  it.each([
    { type: 'slack' as const, fill: fillSlack },
    { type: 'email' as const, fill: fillEmail },
    { type: 'webhook' as const, fill: fillWebhook },
  ])('sends only config keys the API reads for a $type notifier', async ({ type, fill }) => {
    const { onSubmit, user } = renderForm()

    await fill(user)
    await submit(user)

    const sent = Object.keys(submitted(onSubmit).config)
    const known = [...BACKEND_CONFIG_KEYS[type], ...ALLOWED_EXTRA_CONFIG_KEYS[type]]
    expect(sent.filter((key) => !known.includes(key))).toEqual([])
    for (const key of BACKEND_CONFIG_KEYS[type]) {
      expect(sent).toContain(key)
    }
  })

  it('omits blank secrets so saving does not wipe the stored ones', async () => {
    const { onSubmit, user } = renderForm(existingEmailNotifier)

    await user.click(screen.getByRole('button', { name: /^update$/i }))

    expect(submitted(onSubmit)).not.toHaveProperty('smtp_password')
  })

  // The read direction: `notifierToResponse` emits `smtp_username`/`smtp_from`/
  // `smtp_to`, so pre-filling from `smtp_user`/`from_email`/`to_email` left the
  // fields blank and an edit wrote those settings away.
  it('pre-fills an existing email notifier from the keys the API emits', () => {
    renderForm(existingEmailNotifier)

    expect(field.smtpHost()).toHaveValue('smtp.example.com')
    expect(field.smtpPort()).toHaveValue(465)
    expect(field.smtpUsername()).toHaveValue('mailer@example.com')
    expect(field.smtpFrom()).toHaveValue('backups@example.com')
    expect(field.smtpTo()).toHaveValue('admin@example.com, oncall@example.com')
    expect(field.smtpUseTLS()).toBeChecked()
    // The API returns `***REDACTED***`; echoing it back would save it verbatim.
    expect(field.smtpPassword()).toHaveValue('')
  })

  it('pre-fills an existing Slack notifier from config.url', () => {
    renderForm({
      name: 'alerts',
      type: 'slack',
      on_success: false,
      config: { url: 'https://hooks.slack.com/services/T/B/x', channel: '#backups' },
      enabled: true,
      created_at: '',
      updated_at: '',
    })

    expect(field.slackUrl()).toHaveValue('https://hooks.slack.com/services/T/B/x')
    expect(field.channel()).toHaveValue('#backups')
  })

  it('pre-fills webhook method and auth from the nested webhook_auth object', () => {
    renderForm({
      name: 'ops-hook',
      type: 'webhook',
      on_success: false,
      config: {
        url: 'https://api.example.com/webhook',
        webhook_method: 'PUT',
        webhook_auth: { type: 'basic', username: 'backup-bot', password: '***REDACTED***' },
      },
      enabled: true,
      created_at: '',
      updated_at: '',
    })

    expect(field.webhookUrl()).toHaveValue('https://api.example.com/webhook')
    expect(field.webhookMethod()).toHaveValue('PUT')
    expect(field.authUsername()).toHaveValue('backup-bot')
    expect(field.authPassword()).toHaveValue('')
  })

  // No headers editor yet, so an edit must carry the stored ones through rather
  // than dropping a setting the API still reads.
  it('round-trips existing webhook_headers through an edit', async () => {
    const { onSubmit, user } = renderForm({
      name: 'ops-hook',
      type: 'webhook',
      on_success: false,
      config: {
        url: 'https://api.example.com/webhook',
        webhook_method: 'POST',
        webhook_headers: { 'X-Env': 'prod' },
      },
      enabled: true,
      created_at: '',
      updated_at: '',
    })

    await user.click(screen.getByRole('button', { name: /^update$/i }))

    expect(submitted(onSubmit).config).toEqual({
      url: 'https://api.example.com/webhook',
      webhook_method: 'POST',
      webhook_headers: { 'X-Env': 'prod' },
    })
  })
})
