import { Card, CardContent } from '@/components/ui/card'
import type { StatusTone } from '@/lib/status'
import { cn } from '@/lib/utils'
import type { LucideIcon } from 'lucide-react'
import { useId, type ReactNode } from 'react'

/**
 * Tone surfaces for the icon chip, straight off the semantic tokens. Tailwind
 * cannot build a class name at runtime, so the four tones are spelled out.
 */
const toneSurface: Record<StatusTone, string> = {
  neutral: 'bg-muted text-muted-foreground',
  success: 'bg-success-subtle text-success',
  warning: 'bg-warning-subtle text-warning',
  info: 'bg-info-subtle text-info',
  danger: 'bg-danger-subtle text-danger',
}

interface MetricCardProps {
  label: string
  /** A number, or an `<UnknownValue />` when the daemon reported nothing. */
  value: ReactNode
  /** One line of context under the figure — what it counts, or why it is missing. */
  hint?: ReactNode
  tone?: StatusTone
  icon: LucideIcon
}

/**
 * A headline figure on the Overview.
 *
 * `ui/stat-card` is the plain version used elsewhere; this one exists because
 * these four cards need a tone, a supporting line, and a value that may be a
 * node rather than a number. Colour is never the whole signal — the label and
 * the hint say the same thing in words.
 */
export function MetricCard({ label, value, hint, tone = 'neutral', icon: Icon }: MetricCardProps) {
  const labelId = useId()

  return (
    <Card>
      <CardContent className="flex items-start justify-between gap-3 p-5">
        <div className="min-w-0 space-y-1">
          <p
            id={labelId}
            className="text-xs font-medium uppercase tracking-wider text-muted-foreground"
          >
            {label}
          </p>
          {/*
            Mono for anything the daemon produced: sans frames, mono reports.
            The figure names itself, so a screen reader reaching it out of
            order still hears what it counts.
          */}
          <p
            aria-labelledby={labelId}
            className="text-metric font-mono font-semibold tabular-nums tracking-tight"
          >
            {value}
          </p>
          {hint && <p className="text-xs leading-snug text-muted-foreground">{hint}</p>}
        </div>
        <span className={cn('shrink-0 rounded-md p-2', toneSurface[tone])}>
          <Icon aria-hidden="true" className="h-4 w-4" />
        </span>
      </CardContent>
    </Card>
  )
}
