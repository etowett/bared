import {
  Ban,
  CalendarClock,
  Circle,
  CircleCheck,
  CircleSlash,
  CircleX,
  Clock,
  Database,
  LoaderCircle,
  Play,
  Terminal,
  type LucideIcon,
} from 'lucide-react'
import type { Target } from '@/types'

/**
 * The four semantic tones defined in `styles/globals.css`, plus neutral.
 *
 * A tone is only ever half the signal: `StatusBadge` renders a glyph and a word
 * alongside the tint, so a status stays readable with the colour removed.
 */
export type StatusTone = 'neutral' | 'success' | 'warning' | 'info' | 'danger'

export type JobState =
  'running' | 'idle' | 'queued' | 'completed' | 'failed' | 'cancelled' | 'cancelling'

export type JobTrigger = 'manual' | 'schedule' | 'api'

/** What a badge needs to render, independent of what it is describing. */
export interface StatusDescriptor {
  tone: StatusTone
  label: string
  icon: LucideIcon
  /** Spin the glyph — reserved for states where work is genuinely in flight. */
  spin?: boolean
}

/**
 * A backup target's health, in the order an operator cares about it.
 *
 * `unknown` is not a health state — it means the daemon did not report one, and
 * is rendered as the old liveness label rather than a reassuring green tick.
 */
export type TargetHealth = 'running' | 'failing' | 'overdue' | 'healthy' | 'never' | 'unknown'

/** The subjects `describeStatus` knows how to render. */
export type StatusKind = 'job' | 'enabled' | 'database' | 'trigger' | 'target'

const jobStateDescriptors: Record<JobState, StatusDescriptor> = {
  running: { tone: 'info', label: 'Running', icon: LoaderCircle, spin: true },
  queued: { tone: 'warning', label: 'Queued', icon: Clock },
  completed: { tone: 'success', label: 'Completed', icon: CircleCheck },
  failed: { tone: 'danger', label: 'Failed', icon: CircleX },
  cancelling: { tone: 'warning', label: 'Cancelling', icon: CircleSlash },
  cancelled: { tone: 'neutral', label: 'Cancelled', icon: CircleSlash },
  idle: { tone: 'neutral', label: 'Idle', icon: Circle },
}

const triggerDescriptors: Record<JobTrigger, StatusDescriptor> = {
  schedule: { tone: 'info', label: 'Scheduled', icon: CalendarClock },
  manual: { tone: 'warning', label: 'Manual', icon: Play },
  api: { tone: 'neutral', label: 'API', icon: Terminal },
}

const targetHealthDescriptors: Record<TargetHealth, StatusDescriptor> = {
  running: { tone: 'info', label: 'Running', icon: LoaderCircle, spin: true },
  failing: { tone: 'danger', label: 'Failing', icon: CircleX },
  overdue: { tone: 'warning', label: 'Overdue', icon: Clock },
  healthy: { tone: 'success', label: 'Healthy', icon: CircleCheck },
  never: { tone: 'neutral', label: 'Never run', icon: Circle },
  unknown: { tone: 'neutral', label: 'Idle', icon: Circle },
}

/**
 * Reduces a target's health fields to the one thing worth showing in a cell.
 *
 * Work in flight wins, because it is the most recent truth; after that the
 * order is how much it should worry an operator. An older daemon omits these
 * fields entirely, and the answer then is `unknown` — reporting "Healthy" from
 * an absent field would be inventing a claim the backend never made.
 */
export function describeTargetHealth(
  target: Pick<Target, 'is_running' | 'last_backup_status' | 'overdue'>
): TargetHealth {
  if (target.is_running) return 'running'
  if (target.last_backup_status === undefined) return 'unknown'
  if (target.last_backup_status === 'failed') return 'failing'
  if (target.overdue) return 'overdue'
  if (target.last_backup_status === 'never') return 'never'
  return 'healthy'
}

/**
 * Resolves the descriptor for a known subject.
 *
 * Adding a subject — config source (YAML/DB), storage backend, notifier
 * channel — means adding a `StatusKind` and a branch here. Call sites keep the
 * same `<StatusBadge kind=… status=… />` shape and inherit the tone tokens, so
 * no page ever hand-rolls a status colour again.
 */
export function describeStatus(kind: StatusKind, status: string | boolean): StatusDescriptor {
  switch (kind) {
    case 'job':
      return jobStateDescriptors[status as JobState] ?? jobStateDescriptors.idle
    case 'enabled':
      return status
        ? { tone: 'success', label: 'Enabled', icon: CircleCheck }
        : { tone: 'neutral', label: 'Disabled', icon: Ban }
    case 'trigger':
      return triggerDescriptors[status as JobTrigger] ?? triggerDescriptors.api
    case 'target':
      return targetHealthDescriptors[status as TargetHealth] ?? targetHealthDescriptors.unknown
    case 'database':
      // A database engine is an attribute, not a health signal, so it stays
      // neutral — colour is reserved for things an operator must act on.
      return { tone: 'neutral', label: String(status), icon: Database }
  }
}
