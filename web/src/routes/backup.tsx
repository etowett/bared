import { createFileRoute } from '@tanstack/react-router'
import { useState, useMemo } from 'react'
import { useTargets } from '@/hooks/useTargets'
import { useJobs } from '@/hooks/useJobs'
import { TargetList } from '@/components/TargetList'
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

export const Route = createFileRoute('/backup')({
  component: BackupPage,
})

function BackupPage() {
  const { data: dashboard } = useTargets()
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [targetFilter, setTargetFilter] = useState<string>('')

  const { data: jobsData, isLoading } = useJobs(statusFilter !== 'all' ? { status: statusFilter } : undefined)

  // Filter for backup jobs only and by target name
  const backupJobs = useMemo(() => {
    let jobs = (jobsData?.jobs || []).filter((job) => job.type === 'backup')

    if (targetFilter) {
      jobs = jobs.filter((job) => job.target.toLowerCase().includes(targetFilter.toLowerCase()))
    }

    return jobs
  }, [jobsData?.jobs, targetFilter])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold">Backup</h2>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Backup Targets</CardTitle>
        </CardHeader>
        <CardContent>
          <TargetList targets={dashboard?.targets || []} />
        </CardContent>
      </Card>

      {/* Job History Section */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            Backup Job History ({backupJobs.length} Job{backupJobs.length !== 1 ? 's' : ''})
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
