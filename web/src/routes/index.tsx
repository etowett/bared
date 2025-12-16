import { createFileRoute } from '@tanstack/react-router'
import { useDashboard } from '@/hooks/useDashboard'
import { StatCard } from '@/components/ui/stat-card'
import { formatBytes } from '@/lib/utils'

export const Route = createFileRoute('/')({
  component: OverviewPage,
})

function OverviewPage() {
  const { data: dashboard, isLoading } = useDashboard()

  if (isLoading) {
    return <div className="text-center py-12 text-muted-foreground">Loading dashboard...</div>
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
