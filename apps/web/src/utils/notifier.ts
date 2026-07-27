import type { Notifier, NotifierConfigResponse } from '@/types'

/**
 * One-line summary of a notifier for the config list's Configuration column.
 *
 * The keys read here are the ones `notifierToResponse`
 * (apps/api/internal/api/config_handlers.go) actually emits. Reading anything
 * else yields an empty cell that still type-checks, which is exactly what
 * happened: the column read `.method` (the API emits `webhook_method`) and
 * `.to_email` (the API emits `smtp_to`, a list), so both the webhook and the
 * email row rendered blank. The old `WebhookNotifierConfig` type described
 * those absent keys, so the compiler had nothing to object to.
 */
export function describeNotifier(notifier: Notifier): string {
  const config = notifier.config as NotifierConfigResponse

  switch (notifier.type) {
    case 'slack':
      return `Channel: ${config.channel || 'default'}`
    case 'email':
      return `To: ${config.smtp_to?.join(', ') || '—'}`
    case 'webhook':
      return `${config.webhook_method || 'POST'} ${hostOf(config.url)}`
    default:
      return ''
  }
}

/**
 * The hostname of a webhook URL, or the raw value when it will not parse.
 *
 * `ValidateNotifier` requires a URL for webhooks, so a notifier loaded from the
 * database always has one — but a hand-written YAML config reaches the list
 * without passing through that check, and an unguarded `new URL()` would throw
 * during render and blank the entire table.
 */
function hostOf(url: string | undefined): string {
  if (!url) {
    return '—'
  }
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}
