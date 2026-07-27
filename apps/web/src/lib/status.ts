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

/** The subjects `describeStatus` knows how to render. */
export type StatusKind = 'job' | 'enabled' | 'database' | 'trigger'

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
    case 'database':
      // A database engine is an attribute, not a health signal, so it stays
      // neutral — colour is reserved for things an operator must act on.
      return { tone: 'neutral', label: String(status), icon: Database }
  }
}
