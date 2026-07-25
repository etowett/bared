import { StatCard } from '@/components/ui/stat-card'
import { useDashboard } from '@/hooks/useDashboard'
import { formatBytes } from '@/lib/utils'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: OverviewPage,
})

export function OverviewPage() {
  const { data: dashboard, isLoading, isError, error, refetch } = useDashboard()

  if (isLoading) {
    return <div className="text-center py-12 text-muted-foreground">Loading dashboard...</div>
  }

  if (isError) {
    return (
      <div className="text-center py-12">
        <div className="text-destructive mb-4">
          Failed to load dashboard
          {error?.message && `: ${error.message}`}
        </div>
        <button
          onClick={() => refetch()}
          className="px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
        >
          Retry
        </button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-semibold">Overview</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title="Targets" value={dashboard?.targets.length || 0} />
        <StatCard title="Active Jobs" value={dashboard?.active_jobs || 0} />
        <StatCard title="Total Jobs" value={dashboard?.total_jobs || 0} />
        <StatCard title="Storage Used" value={formatBytes(dashboard?.total_storage_bytes)} />
      </div>
    </div>
  )
}
