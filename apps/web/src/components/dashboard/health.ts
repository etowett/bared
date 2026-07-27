import type { Target } from '@/types'
import { formatAge } from '@/utils/age'

/**
 * Fleet triage, derived from the `/api/dashboard` contract and nothing else.
 *
 * One rule runs through every function here: **an absent field is not a good
 * one**. `last_backup_status`, `consecutive_failures` and `overdue` are optional
 * so a newer dashboard stays valid against an older daemon that predates them —
 * so a target that reports none of them is counted as *unreported*, never as
 * healthy. Reporting "Healthy" off a field the backend never sent is the exact
 * failure this contract was designed to avoid.
 */

/** Why a target is in the attention list, most urgent first. */
export type AttentionReason = 'failing' | 'overdue' | 'never'

export interface TargetAttention {
  target: Target
  reason: AttentionReason
  /** One phrase naming the evidence, e.g. "3 failed runs in a row". */
  detail: string
}

export interface FleetSummary {
  total: number
  /** Last backup succeeded, nothing is late, no failure streak. */
  healthy: number
  overdue: number
  /** Targets whose daemon sent no health fields at all. */
  unreported: number
  attention: TargetAttention[]
}

/** The last finished backup failed, or failures have piled up since the last success. */
export function isFailing(target: Target): boolean {
  return target.last_backup_status === 'failed' || (target.consecutive_failures ?? 0) > 0
}

/**
 * The daemon reported no usable health for this target — not a clean bill of
 * health.
 *
 * Two causes, one meaning. An older build sends the fields not at all; a
 * current one sends `last_backup_status: 'unknown'` when it could not read the
 * job history behind them (#134). Either way the dashboard knows nothing, and
 * the banner must stop short of "all current".
 */
export function isUnreported(target: Target): boolean {
  return target.last_backup_status === undefined || target.last_backup_status === 'unknown'
}

/**
 * Healthy is deliberately independent of `is_running`.
 *
 * A target midway through a backup has not stopped being current, and a count
 * that drops by one every time a job starts reads as an incident rather than as
 * routine work.
 */
export function isHealthy(target: Target): boolean {
  return target.last_backup_status === 'success' && !target.overdue && !isFailing(target)
}

/**
 * Orders targets the way an operator triages them, so reading order *is*
 * priority order and the table needs no sort control to be useful.
 */
export function severityRank(target: Target): number {
  if (isFailing(target)) return 0
  if (target.overdue) return 1
  if (target.last_backup_status === 'never') return 2
  if (target.is_running) return 3
  if (isUnreported(target)) return 4
  return 5
}

/** Severity first, then name, so equal-severity rows keep a stable order. */
export function rankTargets(targets: Target[]): Target[] {
  return [...targets].sort(
    (a, b) => severityRank(a) - severityRank(b) || a.name.localeCompare(b.name)
  )
}

function attentionFor(target: Target, now: Date): TargetAttention | null {
  if (isFailing(target)) {
    const streak = target.consecutive_failures ?? 0
    return {
      target,
      reason: 'failing',
      detail: streak > 1 ? `${streak} failed runs in a row` : 'last backup failed',
    }
  }

  if (target.overdue) {
    const age = formatAge(target.last_backup, now)
    return {
      target,
      reason: 'overdue',
      detail: age ? `last success ${age}` : 'scheduled run has not happened',
    }
  }

  if (target.last_backup_status === 'never') {
    return { target, reason: 'never', detail: 'never backed up' }
  }

  return null
}

const attentionOrder: Record<AttentionReason, number> = { failing: 0, overdue: 1, never: 2 }

/**
 * Counts the four figures the Overview leads with, plus the offenders the
 * attention banner links to.
 *
 * @param now Injectable for tests; only used to age the "last success" phrase
 */
export function summarizeFleet(targets: Target[], now: Date = new Date()): FleetSummary {
  const attention = targets
    .map((target) => attentionFor(target, now))
    .filter((entry): entry is TargetAttention => entry !== null)
    .sort(
      (a, b) =>
        attentionOrder[a.reason] - attentionOrder[b.reason] ||
        a.target.name.localeCompare(b.target.name)
    )

  return {
    total: targets.length,
    healthy: targets.filter(isHealthy).length,
    overdue: targets.filter((target) => target.overdue === true).length,
    unreported: targets.filter(isUnreported).length,
    attention,
  }
}
