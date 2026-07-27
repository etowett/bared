import { ActiveJobsCard } from '@/components/dashboard/ActiveJobsCard'
import { AttentionBanner } from '@/components/dashboard/AttentionBanner'
import { BackupSuccessCard } from '@/components/dashboard/BackupSuccessCard'
import { summarizeFleet } from '@/components/dashboard/health'
import { MetricCard } from '@/components/dashboard/MetricCard'
import { NextRunsCard } from '@/components/dashboard/NextRunsCard'
import { TargetHealthTable } from '@/components/dashboard/TargetHealthTable'
import { UnknownValue } from '@/components/dashboard/UnknownValue'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton, StatCardSkeleton } from '@/components/ui/skeleton'
import { useDashboard } from '@/hooks/useDashboard'
import { createFileRoute } from '@tanstack/react-router'
import { Activity, CircleCheck, CircleX, Clock, RefreshCw } from 'lucide-react'

export const Route = createFileRoute('/')({
  component: OverviewPage,
})

/**
 * The operator Overview.
 *
 * It exists to answer one question — "are my backups healthy and current?" —
 * in the order an on-call engineer asks it: what is broken, how much of the
 * fleet is fine, what is about to run, what is running now.
 *
 * Every panel is fed by the `/api/dashboard` contract and nothing is inferred.
 * Where the contract has no answer, the page says so in words rather than
 * printing a zero: `success_rate_7d` is null on any daemon without a persistent
 * job store, and `total_storage_bytes` is never populated at all. That is why
 * there is no storage-growth panel and no recent-activity feed here — the data
 * for them does not exist, and a panel of invented numbers is worse than no
 * panel.
 */
export function OverviewPage() {
  const { data: dashboard, isPending, isError, error, refetch } = useDashboard()

  const targets = dashboard?.targets ?? []
  const fleet = summarizeFleet(targets)

  if (isError) {
    return (
      <div className="space-y-6">
        <OverviewHeader />
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
      </div>
    )
  }

  // First load only. A poll that returns keeps the panels on screen — swapping
  // real numbers for grey blocks every five seconds is worse than a stale one.
  if (isPending) {
    return (
      <div className="space-y-6">
        <OverviewHeader />
        <OverviewSkeleton />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <OverviewHeader />

      <AttentionBanner
        attention={fleet.attention}
        total={fleet.total}
        unreported={fleet.unreported}
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="Healthy targets"
          value={fleet.healthy}
          tone={fleet.total > 0 && fleet.healthy === fleet.total ? 'success' : 'neutral'}
          icon={CircleCheck}
          hint={
            fleet.total === 0
              ? 'No targets configured yet'
              : fleet.unreported > 0
                ? `of ${fleet.total} configured · ${fleet.unreported} report no health data`
                : `of ${fleet.total} configured`
          }
        />
        <MetricCard
          label="Failed in 24h"
          value={dashboard?.failed_jobs_24h ?? <UnknownValue />}
          tone={(dashboard?.failed_jobs_24h ?? 0) > 0 ? 'danger' : 'neutral'}
          icon={CircleX}
          hint={
            dashboard?.failed_jobs_24h === undefined
              ? 'Job history was truncated, so this could not be counted'
              : 'Backup jobs that ended in failure'
          }
        />
        <MetricCard
          label="Overdue"
          value={fleet.overdue}
          tone={fleet.overdue > 0 ? 'warning' : 'neutral'}
          icon={Clock}
          hint="Past the run their schedule was due"
        />
        <MetricCard
          label="Running or queued"
          value={dashboard?.active_jobs ?? 0}
          tone={(dashboard?.active_jobs ?? 0) > 0 ? 'info' : 'neutral'}
          icon={Activity}
          hint="Backup and restore jobs in flight"
        />
      </div>

      {/* `items-start` on purpose: a short card next to a tall table reads
          better than a stretched one with a band of dead space in it. */}
      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-3">
        <BackupSuccessCard
          success_rate_24h={dashboard?.success_rate_24h}
          success_rate_7d={dashboard?.success_rate_7d}
          failed_jobs_24h={dashboard?.failed_jobs_24h}
          total_storage_bytes={dashboard?.total_storage_bytes}
        />
        <div className="lg:col-span-2">
          <TargetHealthTable targets={targets} />
        </div>
      </div>

      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-2">
        <NextRunsCard targets={targets} />
        <ActiveJobsCard activeJobs={dashboard?.active_jobs} />
      </div>
    </div>
  )
}

function OverviewHeader() {
  return (
    <PageHeader
      title="Overview"
      description="Whether your backups are healthy and current, and what the daemon does next."
    />
  )
}

/** Holds the real layout's shape so nothing jumps when the first payload lands. */
function OverviewSkeleton() {
  return (
    <div className="space-y-6" data-testid="overview-skeleton">
      <span className="sr-only" role="status">
        Loading the overview
      </span>
      <Skeleton className="h-14 w-full" />
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCardSkeleton />
        <StatCardSkeleton />
        <StatCardSkeleton />
        <StatCardSkeleton />
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Skeleton className="h-64" />
        <Skeleton className="h-64 lg:col-span-2" />
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Skeleton className="h-48" />
        <Skeleton className="h-48" />
      </div>
    </div>
  )
}
