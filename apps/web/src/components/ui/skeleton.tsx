import { cn } from '@/lib/utils'

/**
 * A placeholder block that holds the space its content will occupy.
 *
 * Use it for the *first* load only. During a background refetch keep the
 * previous data on screen — swapping real rows for grey ones every few seconds
 * is worse than a stale number.
 */
export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      aria-hidden="true"
      className={cn('motion-safe:animate-pulse rounded-md bg-muted', className)}
      {...props}
    />
  )
}

/** A skeleton shaped like the stat cards on the Overview. */
export function StatCardSkeleton() {
  return (
    <div className="rounded-lg border bg-card p-6 shadow-xs">
      <Skeleton className="h-3 w-24" />
      <Skeleton className="mt-4 h-8 w-16" />
    </div>
  )
}

interface TableSkeletonProps {
  rows?: number
  columns?: number
}

/** A skeleton shaped like a `Table`, so the page does not jump when data lands. */
export function TableSkeleton({ rows = 5, columns = 6 }: TableSkeletonProps) {
  return (
    <div className="space-y-3" data-testid="table-skeleton">
      <span className="sr-only" role="status">
        Loading
      </span>
      {Array.from({ length: rows }).map((_, rowIndex) => (
        <div key={rowIndex} className="flex items-center gap-4 border-b pb-3 last:border-0">
          {Array.from({ length: columns }).map((_, columnIndex) => (
            <Skeleton
              key={columnIndex}
              className={cn('h-4 flex-1', columnIndex === 0 && 'max-w-[5rem]')}
            />
          ))}
        </div>
      ))}
    </div>
  )
}
