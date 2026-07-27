import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState, useMemo } from 'react'
import { useJobs } from '@/hooks/useJobs'
import { JobList } from '@/components/JobList'
import { JobDetail } from '@/components/JobDetail'
import type { Job } from '@/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { TableSkeleton } from '@/components/ui/skeleton'
import { ArrowLeft } from 'lucide-react'

export const Route = createLazyFileRoute('/restore/jobs')({
  component: RestoreJobsPage,
})

export function RestoreJobsPage() {
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [targetFilter, setTargetFilter] = useState<string>('')

  // Build filters object with type and optional status
  const filters = useMemo(() => {
    const filterObj: Record<string, string> = { type: 'restore' }
    if (statusFilter !== 'all') {
      filterObj.status = statusFilter
    }
    return filterObj
  }, [statusFilter])

  const { data: jobsData, isPending } = useJobs(filters)

  // Filter by target name (client-side)
  const restoreJobs = useMemo(() => {
    let jobs = jobsData?.jobs || []

    if (targetFilter) {
      jobs = jobs.filter((job) => job.target.toLowerCase().includes(targetFilter.toLowerCase()))
    }

    return jobs
  }, [jobsData?.jobs, targetFilter])

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

      <Card>
        <CardHeader className="flex flex-col gap-4 space-y-0 pb-4 sm:flex-row sm:items-center sm:justify-between">
          <CardTitle>
            {restoreJobs.length} Restore Job{restoreJobs.length !== 1 ? 's' : ''}
          </CardTitle>
          <div className="flex flex-wrap gap-3">
            <Input
              placeholder="Filter by target..."
              value={targetFilter}
              onChange={(e) => setTargetFilter(e.target.value)}
              className="w-full sm:w-[200px]"
            />
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-full sm:w-[180px]">
                <SelectValue placeholder="All Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Status</SelectItem>
                <SelectItem value="queued">Queued</SelectItem>
                <SelectItem value="running">Running</SelectItem>
                <SelectItem value="completed">Completed</SelectItem>
                <SelectItem value="failed">Failed</SelectItem>
                <SelectItem value="cancelled">Cancelled</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {isPending ? (
            <TableSkeleton rows={5} columns={7} />
          ) : restoreJobs.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              No restore jobs found. {targetFilter && 'Try adjusting your filters.'}
            </div>
          ) : (
            <JobList
              jobs={restoreJobs}
              onSelectJob={setSelectedJob}
              selectedJobId={selectedJob?.id}
            />
          )}
        </CardContent>
      </Card>

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
