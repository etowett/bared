import { JobDetail } from '@/components/JobDetail'
import { JobList } from '@/components/JobList'
import { TargetList } from '@/components/TargetList'
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
import { useTargets } from '@/hooks/useTargets'
import type { Job } from '@/types'
import { createLazyFileRoute } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

export const Route = createLazyFileRoute('/backup/')({
  component: BackupPage,
})

export function BackupPage() {
  const { data: dashboard } = useTargets()
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [targetFilter, setTargetFilter] = useState<string>('')
  const [targetSearch, setTargetSearch] = useState<string>('')
  const [typeFilter, setTypeFilter] = useState<string>('all')

  const { data: jobsData, isLoading } = useJobs(
    statusFilter !== 'all' ? { status: statusFilter } : undefined
  )

  const targets = useMemo(() => dashboard?.targets || [], [dashboard?.targets])

  // Derive unique db types for filter dropdown
  const dbTypes = useMemo(() => {
    const types = new Set(targets.map((t) => t.type))
    return Array.from(types).sort()
  }, [targets])

  // Filter targets by search and type
  const filteredTargets = useMemo(() => {
    let result = targets
    if (targetSearch) {
      result = result.filter((t) => t.name.toLowerCase().includes(targetSearch.toLowerCase()))
    }
    if (typeFilter !== 'all') {
      result = result.filter((t) => t.type === typeFilter)
    }
    return result
  }, [targets, targetSearch, typeFilter])

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
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>Backup Targets ({filteredTargets.length})</CardTitle>
          <div className="flex gap-3">
            <Input
              placeholder="Search targets..."
              value={targetSearch}
              onChange={(e) => setTargetSearch(e.target.value)}
              className="w-[200px]"
            />
            <Select value={typeFilter} onValueChange={setTypeFilter}>
              <SelectTrigger className="w-[150px]">
                <SelectValue placeholder="All Types" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Types</SelectItem>
                {dbTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          <TargetList targets={filteredTargets} />
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
