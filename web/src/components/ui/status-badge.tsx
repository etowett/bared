import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { cva, type VariantProps } from 'class-variance-authority'

const statusBadgeVariants = cva('uppercase text-xs font-semibold border', {
  variants: {
    status: {
      running: 'bg-terminal-info/20 text-terminal-cyan border-terminal-info/30 hover:bg-terminal-info/30 dark:bg-terminal-info/10 dark:border-terminal-info/20',
      idle: 'bg-muted text-muted-foreground border-border hover:bg-muted',
      queued: 'bg-terminal-warning/20 text-terminal-yellow border-terminal-warning/30 hover:bg-terminal-warning/30 dark:bg-terminal-warning/10 dark:border-terminal-warning/20',
      completed: 'bg-terminal-success/20 text-terminal-green border-terminal-success/30 hover:bg-terminal-success/30 dark:bg-terminal-success/10 dark:border-terminal-success/20',
      failed: 'bg-terminal-error/20 text-terminal-red border-terminal-error/30 hover:bg-terminal-error/30 dark:bg-terminal-error/10 dark:border-terminal-error/20',
      cancelled: 'bg-muted text-muted-foreground border-border hover:bg-muted',
      cancelling: 'bg-muted text-muted-foreground border-border hover:bg-muted',
    },
  },
  defaultVariants: { status: 'idle' },
})

export interface StatusBadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof statusBadgeVariants> {
  status: 'running' | 'idle' | 'queued' | 'completed' | 'failed' | 'cancelled' | 'cancelling'
}

export function StatusBadge({ status, className, ...props }: StatusBadgeProps) {
  return (
    <Badge className={cn(statusBadgeVariants({ status }), className)} {...props}>
      {status}
    </Badge>
  )
}
