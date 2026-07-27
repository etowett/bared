import { StatusBadge } from '@/components/ui/status-badge'
import type { TargetHealth } from '@/lib/status'
import { cn } from '@/lib/utils'
import { CircleCheck, CircleHelp } from 'lucide-react'
import type { AttentionReason, TargetAttention } from './health'

interface AttentionBannerProps {
  attention: TargetAttention[]
  total: number
  /** Targets whose daemon reported no usable health. Never counted as healthy. */
  unreported: number
}

/** The attention reasons map onto the badge vocabulary `StatusBadge` already speaks. */
const reasonHealth: Record<AttentionReason, TargetHealth> = {
  failing: 'failing',
  overdue: 'overdue',
  never: 'never',
}

/**
 * The first thing on the page, and on a good day the only thing worth reading.
 *
 * Each offender is a link to its own row in the health table below, so the
 * banner routes rather than merely reports. When nothing needs attention it
 * shrinks to a single line — and if any target is unreported, that line stops
 * short of claiming everything is fine.
 */
export function AttentionBanner({ attention, total, unreported }: AttentionBannerProps) {
  if (attention.length === 0) {
    if (total === 0) return null

    const allAccountedFor = unreported === 0

    return (
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border bg-card px-4 py-3">
        <StatusBadge
          kind="custom"
          tone={allAccountedFor ? 'success' : 'neutral'}
          label={allAccountedFor ? 'All current' : 'No problems reported'}
          icon={allAccountedFor ? CircleCheck : CircleHelp}
        />
        <p className="text-sm text-muted-foreground">
          {allAccountedFor
            ? `All ${total} configured ${total === 1 ? 'target is' : 'targets are'} backed up and on schedule.`
            : `${unreported} of ${total} ${unreported === 1 ? 'target reports' : 'targets report'} no usable health data — an older daemon, or one that cannot read its job history.${
                unreported < total ? ' The rest are on schedule.' : ''
              }`}
        </p>
      </div>
    )
  }

  const anyFailing = attention.some((entry) => entry.reason === 'failing')
  const count = attention.length

  return (
    <section
      aria-labelledby="attention-heading"
      className={cn(
        'rounded-lg border p-4',
        anyFailing ? 'border-danger/25 bg-danger-subtle' : 'border-warning/25 bg-warning-subtle'
      )}
    >
      <h3
        id="attention-heading"
        className={cn(
          'text-section-title font-semibold',
          anyFailing ? 'text-danger' : 'text-warning'
        )}
      >
        {count} {count === 1 ? 'target needs' : 'targets need'} attention
      </h3>
      <p className="mt-1 text-sm text-muted-foreground">
        Pick one to jump to its row in the health table.
      </p>

      <ul className="mt-3 flex flex-wrap gap-2">
        {attention.map(({ target, reason, detail }) => (
          <li key={target.name}>
            <a
              href={`#target-${target.name}`}
              className="flex flex-wrap items-center gap-2 rounded-md border bg-card px-2.5 py-1.5 text-sm transition-colors hover:bg-accent focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            >
              <StatusBadge kind="target" status={reasonHealth[reason]} />
              <span className="font-medium">{target.name}</span>
              <span className="text-muted-foreground">{detail}</span>
            </a>
          </li>
        ))}
      </ul>
    </section>
  )
}
