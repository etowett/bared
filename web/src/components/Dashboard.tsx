import { useState } from 'react'
import { useDashboard } from '../hooks/useDashboard'
import { useJobs } from '../hooks/useJobs'
import type { Job } from '../types'
import { JobDetail } from './JobDetail'
import { JobList } from './JobList'
import { RestoreForm } from './RestoreForm'
import { TargetList } from './TargetList'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatCard } from '@/components/ui/stat-card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatBytes } from '@/lib/utils'

export function Dashboard() {
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('')

  const { data: dashboard, isLoading: dashboardLoading } = useDashboard()
  const { data: jobsData, isLoading: jobsLoading } = useJobs(
    statusFilter ? { status: statusFilter } : undefined
  )

  if (dashboardLoading) {
    return <div className="text-center py-12 text-muted-foreground">Loading dashboard...</div>
  }

  return (
    <div className="space-y-8">
      {/* Overview Section */}
      <section id="overview" className="scroll-mt-20">
        <h2 className="text-2xl font-semibold mb-4">Overview</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          <StatCard title="Targets" value={dashboard?.targets.length || 0} />
          <StatCard title="Active Jobs" value={dashboard?.active_jobs || 0} />
          <StatCard title="Total Jobs" value={dashboard?.total_jobs || 0} />
          <StatCard title="Storage Used" value={formatBytes(dashboard?.total_storage_bytes)} />
        </div>
      </section>

      {/* Restore Section */}
      <section id="restore" className="scroll-mt-20">
        <Card>
          <CardHeader>
            <CardTitle>Restore Database</CardTitle>
          </CardHeader>
          <CardContent>
            <RestoreForm />
          </CardContent>
        </Card>
      </section>

      {/* Targets Section */}
      <section id="targets" className="scroll-mt-20">
        <Card>
          <CardHeader>
            <CardTitle>Backup Targets</CardTitle>
          </CardHeader>
          <CardContent>
            <TargetList targets={dashboard?.targets || []} />
          </CardContent>
        </Card>
      </section>

      {/* Jobs Section */}
      <section id="jobs" className="scroll-mt-20">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
            <CardTitle>Jobs</CardTitle>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="All Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="queued">Queued</SelectItem>
                <SelectItem value="running">Running</SelectItem>
                <SelectItem value="completed">Completed</SelectItem>
                <SelectItem value="failed">Failed</SelectItem>
                <SelectItem value="cancelled">Cancelled</SelectItem>
              </SelectContent>
            </Select>
          </CardHeader>
          <CardContent>
            {jobsLoading ? (
              <div className="text-center py-6 text-muted-foreground">Loading jobs...</div>
            ) : (
              <JobList
                jobs={jobsData?.jobs || []}
                onSelectJob={setSelectedJob}
                selectedJobId={selectedJob?.id}
              />
            )}
          </CardContent>
        </Card>
      </section>

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
