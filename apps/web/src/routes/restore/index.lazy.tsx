import { JobDetail } from '@/components/JobDetail'
import { JobHistoryTable } from '@/components/JobHistoryTable'
import { RestoreForm } from '@/components/RestoreForm'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PageHeader } from '@/components/ui/page-header'
import { useRestoreTargets } from '@/hooks/useRestoreTargets'
import type { JobSearch } from '@/lib/job-search'
import type { Job } from '@/types'
import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { History } from 'lucide-react'
import { useState } from 'react'

export const Route = createLazyFileRoute('/restore/')({
  component: RestorePage,
})

export function RestorePage() {
  const navigate = useNavigate()
  const search = Route.useSearch() as JobSearch
  const { data: restoreTargets } = useRestoreTargets()
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)

  return (
    <div className="space-y-6">
      <PageHeader
        title="Restore"
        description="Write a stored backup back into a database. This overwrites the target."
        actions={
          <Button asChild variant="outline">
            <Link to="/restore/jobs">
              <History aria-hidden="true" className="mr-2 h-4 w-4" />
              Job history
            </Link>
          </Button>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle>Restore Database</CardTitle>
        </CardHeader>
        <CardContent>
          <RestoreForm />
        </CardContent>
      </Card>

      <JobHistoryTable
        title="Restore Job History"
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
