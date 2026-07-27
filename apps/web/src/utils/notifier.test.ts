import { describe, expect, it } from 'vitest'
import type { Notifier, NotifierType } from '../types'
import { describeNotifier } from './notifier'

function notifier(type: NotifierType, config: Record<string, unknown>): Notifier {
  return {
    name: 'n',
    type,
    on_success: false,
    config,
    enabled: true,
    created_at: '',
    updated_at: '',
  }
}

describe('describeNotifier', () => {
  // The exact payload `notifierToResponse` emits for a webhook. The list page
  // used to read `.method`, which the API has never emitted, so this cell was
  // blank for every webhook notifier in the dashboard.
  it('reads webhook_method, not method', () => {
    const summary = describeNotifier(
      notifier('webhook', {
        url: 'https://api.example.com/webhook',
        webhook_method: 'PUT',
        webhook_auth: { type: 'basic', username: 'bot', password: '***REDACTED***' },
      })
    )

    expect(summary).toBe('PUT api.example.com')
  })

  it('falls back to POST when the API omits webhook_method', () => {
    expect(describeNotifier(notifier('webhook', { url: 'https://api.example.com/hook' }))).toBe(
      'POST api.example.com'
    )
  })

  // A YAML-sourced notifier never passes through ValidateNotifier, so the URL
  // can be missing or malformed. `new URL()` on it would throw during render
  // and blank the whole table rather than one cell.
  it('survives a missing or unparseable webhook url', () => {
    expect(describeNotifier(notifier('webhook', { webhook_method: 'POST' }))).toBe('POST —')
    expect(describeNotifier(notifier('webhook', { url: 'not a url' }))).toBe('POST not a url')
  })

  // The API emits `smtp_to`, a list. The list page read `to_email`, which does
  // not exist on the wire, so every email notifier showed "To:" and nothing.
  it('reads smtp_to, not to_email', () => {
    const summary = describeNotifier(
      notifier('email', {
        smtp_host: 'smtp.example.com',
        smtp_to: ['ops@example.com', 'oncall@example.com'],
      })
    )

    expect(summary).toBe('To: ops@example.com, oncall@example.com')
  })

  it('handles an email notifier with no recipients', () => {
    expect(describeNotifier(notifier('email', { smtp_host: 'smtp.example.com' }))).toBe('To: —')
  })

  it('describes a slack notifier by channel, defaulting when unset', () => {
    expect(describeNotifier(notifier('slack', { url: 'https://hooks.slack.com/x' }))).toBe(
      'Channel: default'
    )
    expect(describeNotifier(notifier('slack', { channel: '#backups' }))).toBe('Channel: #backups')
  })
})
