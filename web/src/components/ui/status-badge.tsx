import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { cva, type VariantProps } from 'class-variance-authority'

const statusBadgeVariants = cva('uppercase text-xs font-semibold', {
  variants: {
    status: {
      running: 'bg-blue-100 text-blue-800 hover:bg-blue-100',
      idle: 'bg-gray-100 text-gray-800 hover:bg-gray-100',
      queued: 'bg-indigo-100 text-indigo-800 hover:bg-indigo-100',
      completed: 'bg-green-100 text-green-800 hover:bg-green-100',
      failed: 'bg-red-100 text-red-800 hover:bg-red-100',
      cancelled: 'bg-gray-100 text-gray-800 hover:bg-gray-100',
      cancelling: 'bg-gray-100 text-gray-800 hover:bg-gray-100',
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
