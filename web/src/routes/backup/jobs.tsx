import { JobDetail } from '@/components/JobDetail'
import { JobList } from '@/components/JobList'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useJobs } from '@/hooks/useJobs'
import type { Job } from '@/types'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useMemo, useState } from 'react'

export const Route = createFileRoute('/backup/jobs')({
  component: BackupJobsPage,
})

function BackupJobsPage() {
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [targetFilter, setTargetFilter] = useState<string>('')

  // Build filters object with type and optional status
  const filters = useMemo(() => {
    const filterObj: Record<string, string> = { type: 'backup' }
    if (statusFilter !== 'all') {
      filterObj.status = statusFilter
    }
    return filterObj
  }, [statusFilter])

  const { data: jobsData, isLoading } = useJobs(filters)

  // Filter by target name (client-side)
  const backupJobs = useMemo(() => {
    let jobs = jobsData?.jobs || []

    if (targetFilter) {
      jobs = jobs.filter((job) => job.target.toLowerCase().includes(targetFilter.toLowerCase()))
    }

    return jobs
  }, [jobsData?.jobs, targetFilter])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold">Backup Job History</h2>
        <Button asChild variant="outline">
          <Link to="/backup">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Targets
          </Link>
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {backupJobs.length} Backup Job{backupJobs.length !== 1 ? 's' : ''}
          </CardTitle>
          <div className="flex gap-3">
            <Input
              placeholder="Filter by target..."
              value={targetFilter}
              onChange={(e) => setTargetFilter(e.target.value)}
              className="w-[200px]"
            />
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-[180px]">
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
          {isLoading ? (
            <div className="text-center py-6 text-muted-foreground">Loading jobs...</div>
          ) : backupJobs.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              No backup jobs found. {targetFilter && 'Try adjusting your filters.'}
            </div>
          ) : (
            <JobList
              jobs={backupJobs}
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
