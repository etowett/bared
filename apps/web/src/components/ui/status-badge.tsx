import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import {
  describeStatus,
  type JobState,
  type JobTrigger,
  type StatusDescriptor,
  type StatusKind,
  type StatusTone,
} from '@/lib/status'
import { cva } from 'class-variance-authority'
import { CircleDot, type LucideIcon } from 'lucide-react'

export type { JobState, JobTrigger, StatusTone }

const toneVariants = cva(
  'inline-flex items-center gap-1.5 border px-2 py-1 text-[0.6875rem] font-semibold uppercase leading-none tracking-wide',
  {
    variants: {
      tone: {
        neutral: 'bg-muted text-muted-foreground border-border hover:bg-muted',
        success: 'bg-success-subtle text-success border-success/25 hover:bg-success-subtle',
        warning: 'bg-warning-subtle text-warning border-warning/25 hover:bg-warning-subtle',
        info: 'bg-info-subtle text-info border-info/25 hover:bg-info-subtle',
        danger: 'bg-danger-subtle text-danger border-danger/25 hover:bg-danger-subtle',
      },
    },
    defaultVariants: { tone: 'neutral' },
  }
)

type BadgeAttributes = Omit<React.HTMLAttributes<HTMLDivElement>, 'children'>

export type StatusBadgeProps = BadgeAttributes &
  (
    | { kind?: 'job'; status: JobState }
    | { kind: 'enabled'; status: boolean }
    | { kind: 'trigger'; status: JobTrigger }
    | { kind: 'database'; status: string }
    /** Escape hatch for subjects `describeStatus` does not cover yet. */
    | { kind: 'custom'; status?: never; tone: StatusTone; label: string; icon?: LucideIcon }
  )

/**
 * The one badge for every status in the dashboard.
 *
 * It always renders a glyph *and* a word, so no state is carried by colour
 * alone, and every tint comes from the semantic tokens in `globals.css`.
 */
export function StatusBadge({ className, ...props }: StatusBadgeProps) {
  const {
    kind,
    status,
    tone: customTone,
    label: customLabel,
    icon: customIcon,
    ...rest
  } = props as BadgeAttributes & {
    kind?: StatusKind | 'custom'
    status?: string | boolean
    tone?: StatusTone
    label?: string
    icon?: LucideIcon
  }

  const descriptor: StatusDescriptor =
    kind === 'custom'
      ? { tone: customTone ?? 'neutral', label: customLabel ?? '', icon: customIcon ?? CircleDot }
      : describeStatus(kind ?? 'job', status ?? '')

  const { tone, label, icon: Icon, spin } = descriptor

  return (
    <Badge className={cn(toneVariants({ tone }), className)} {...rest}>
      <Icon
        aria-hidden="true"
        className={cn('h-3 w-3 shrink-0', spin && 'motion-safe:animate-spin')}
      />
      {label}
    </Badge>
  )
}
