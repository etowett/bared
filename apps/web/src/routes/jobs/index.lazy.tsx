import { JobHistoryTable } from '@/components/JobHistoryTable'
import { PageHeader } from '@/components/ui/page-header'
import { useRestoreTargets } from '@/hooks/useRestoreTargets'
import { useTargets } from '@/hooks/useTargets'
import type { JobSearch } from '@/lib/job-search'
import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'

export const Route = createLazyFileRoute('/jobs/')({
  component: UnifiedJobsPage,
})

export function UnifiedJobsPage() {
  const navigate = useNavigate()
  const search = Route.useSearch() as JobSearch
  const { data: targetsData } = useTargets()
  const { data: restoreTargetsData } = useRestoreTargets()

  // This page lists both kinds of job, so the target filter has to offer both
  // kinds of name — a restore job's target is a restore target.
  const targetOptions = useMemo(
    () => [
      ...(targetsData?.targets ?? []).map((target) => target.name),
      ...(restoreTargetsData?.restore_targets ?? []).map((target) => target.name),
    ],
    [targetsData?.targets, restoreTargetsData?.restore_targets]
  )

  return (
    <div className="space-y-6">
      <PageHeader
        title="All Jobs"
        description="Every backup and restore the daemon has run, newest first."
      />

      <JobHistoryTable
        title="Jobs"
        search={search}
        onSearchChange={(next) => navigate({ to: '.', search: { ...search, ...next } })}
        targetOptions={targetOptions}
      />
    </div>
  )
}
