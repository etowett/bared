import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { StatCardSkeleton } from '@/components/ui/skeleton'
import { StatCard } from '@/components/ui/stat-card'
import { useDashboard } from '@/hooks/useDashboard'
import { formatBytes } from '@/lib/utils'
import { createFileRoute } from '@tanstack/react-router'
import { RefreshCw } from 'lucide-react'

export const Route = createFileRoute('/')({
  component: OverviewPage,
})

export function OverviewPage() {
  const { data: dashboard, isPending, isError, error, refetch } = useDashboard()

  return (
    <div className="space-y-6">
      <PageHeader
        title="Overview"
        description="What the daemon is doing right now, and what it has stored."
      />

      {isError ? (
        <div className="rounded-lg border border-danger/25 bg-danger-subtle p-6">
          <p className="font-medium text-danger">
            Could not load the dashboard{error?.message ? `: ${error.message}` : '.'}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            The daemon may be restarting. Try again in a moment.
          </p>
          <Button variant="outline" size="sm" className="mt-4 gap-1.5" onClick={() => refetch()}>
            <RefreshCw aria-hidden="true" className="h-3.5 w-3.5" />
            Try again
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {/* First load only — a poll that returns keeps the numbers on screen. */}
          {isPending ? (
            <>
              <StatCardSkeleton />
              <StatCardSkeleton />
              <StatCardSkeleton />
              <StatCardSkeleton />
            </>
          ) : (
            <>
              <StatCard title="Targets" value={dashboard?.targets.length || 0} />
              <StatCard title="Active Jobs" value={dashboard?.active_jobs || 0} />
              <StatCard title="Total Jobs" value={dashboard?.total_jobs || 0} />
              <StatCard title="Storage Used" value={formatBytes(dashboard?.total_storage_bytes)} />
            </>
          )}
        </div>
      )}
    </div>
  )
}
