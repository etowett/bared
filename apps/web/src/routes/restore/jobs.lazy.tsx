import { JobDetail } from '@/components/JobDetail'
import { JobHistoryTable } from '@/components/JobHistoryTable'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { useRestoreTargets } from '@/hooks/useRestoreTargets'
import type { JobSearch } from '@/lib/job-search'
import type { Job } from '@/types'
import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'

export const Route = createLazyFileRoute('/restore/jobs')({
  component: RestoreJobsPage,
})

export function RestoreJobsPage() {
  const navigate = useNavigate()
  const search = Route.useSearch() as JobSearch
  const { data: restoreTargets } = useRestoreTargets()
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)

  return (
    <div className="space-y-6">
      <PageHeader
        breadcrumbs={[{ label: 'Restore', to: '/restore' }, { label: 'Job history' }]}
        title="Restore Job History"
        description="Every restore this daemon has attempted, whatever the outcome."
        actions={
          <Button asChild variant="outline">
            <Link to="/restore">
              <ArrowLeft aria-hidden="true" className="mr-2 h-4 w-4" />
              Back to Restore
            </Link>
          </Button>
        }
      />

      <JobHistoryTable
        title="Restore Jobs"
        type="restore"
        search={search}
        onSearchChange={(next) => navigate({ to: '.', search: { ...search, ...next } })}
        targetOptions={(restoreTargets?.restore_targets ?? []).map((target) => target.name)}
        onSelectJob={setSelectedJob}
        selectedJobId={selectedJob?.id}
        emptyMessage="No restore jobs found."
      />

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
