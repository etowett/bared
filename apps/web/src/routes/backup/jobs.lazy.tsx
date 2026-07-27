import { JobDetail } from '@/components/JobDetail'
import { JobHistoryTable } from '@/components/JobHistoryTable'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { useTargets } from '@/hooks/useTargets'
import type { JobSearch } from '@/lib/job-search'
import type { Job } from '@/types'
import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'

export const Route = createLazyFileRoute('/backup/jobs')({
  component: BackupJobsPage,
})

export function BackupJobsPage() {
  const navigate = useNavigate()
  const search = Route.useSearch() as JobSearch
  const { data: targetsData } = useTargets()
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)

  return (
    <div className="space-y-6">
      <PageHeader
        breadcrumbs={[{ label: 'Backup', to: '/backup' }, { label: 'Job history' }]}
        title="Backup Job History"
        description="Every backup this daemon has attempted, whatever the outcome."
        actions={
          <Button asChild variant="outline">
            <Link to="/backup">
              <ArrowLeft aria-hidden="true" className="mr-2 h-4 w-4" />
              Back to Targets
            </Link>
          </Button>
        }
      />

      <JobHistoryTable
        title="Backup Jobs"
        type="backup"
        search={search}
        onSearchChange={(next) => navigate({ to: '.', search: { ...search, ...next } })}
        targetOptions={(targetsData?.targets ?? []).map((target) => target.name)}
        onSelectJob={setSelectedJob}
        selectedJobId={selectedJob?.id}
        emptyMessage="No backup jobs found."
      />

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
