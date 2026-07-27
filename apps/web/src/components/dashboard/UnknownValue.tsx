import { cn } from '@/lib/utils'

/**
 * "unknown" — the daemon could have an answer but did not send one.
 * "unavailable" — the daemon does not track this figure at all.
 */
type UnknownKind = 'unknown' | 'unavailable'

interface UnknownValueProps {
  kind?: UnknownKind
  /** Optional hover text. Never put anything load-bearing here — say it in the panel. */
  title?: string
  className?: string
}

/**
 * The one way this dashboard renders a figure it does not have.
 *
 * Every real number on the Overview is solid, foreground-coloured and
 * `tabular-nums`; a missing one is lowercase, muted and sits on a dashed rule —
 * an empty slot, not a value. That difference is the point: on a backup
 * dashboard "we have no sample" and "0%" are opposite claims, and a reader who
 * cannot tell them apart will act on the wrong one. `success_rate_7d` is null
 * on any daemon without a persistent job store, which is the common case.
 */
export function UnknownValue({ kind = 'unknown', title, className }: UnknownValueProps) {
  return (
    <span
      title={title}
      className={cn(
        'border-b border-dashed border-muted-foreground/50 font-mono lowercase text-muted-foreground',
        className
      )}
    >
      {kind}
    </span>
  )
}
