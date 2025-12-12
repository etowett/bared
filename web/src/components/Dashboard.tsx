import { useState } from 'react'
import { clearAuth } from '../api/client'
import { useDashboard } from '../hooks/useDashboard'
import { useJobs } from '../hooks/useJobs'
import type { Job } from '../types'
import { JobDetail } from './JobDetail'
import { JobList } from './JobList'
import { RestoreForm } from './RestoreForm'
import { TargetList } from './TargetList'
import { Button } from '@/components/ui/button'
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

interface DashboardProps {
  onLogout: () => void
}

export function Dashboard({ onLogout }: DashboardProps) {
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('')

  const { data: dashboard, isLoading: dashboardLoading } = useDashboard()
  const { data: jobsData, isLoading: jobsLoading } = useJobs(
    statusFilter ? { status: statusFilter } : undefined
  )

  const handleLogout = () => {
    clearAuth()
    onLogout()
  }

  if (dashboardLoading) {
    return (
      <div className="min-h-screen p-8">
        <div className="text-center py-12 text-muted-foreground">Loading dashboard...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen p-8 bg-slate-50">
      <header className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold text-foreground">BareD - Backup Dashboard</h1>
        <Button onClick={handleLogout} variant="secondary">
          Logout
        </Button>
      </header>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <StatCard title="Targets" value={dashboard?.targets.length || 0} />
        <StatCard title="Active Jobs" value={dashboard?.active_jobs || 0} />
        <StatCard title="Total Jobs" value={dashboard?.total_jobs || 0} />
        <StatCard title="Storage Used" value={formatBytes(dashboard?.total_storage_bytes)} />
      </div>

      <div className="flex flex-col gap-8">
        <Card>
          <CardHeader>
            <CardTitle>Restore Database</CardTitle>
          </CardHeader>
          <CardContent>
            <RestoreForm />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Backup Targets</CardTitle>
          </CardHeader>
          <CardContent>
            <TargetList targets={dashboard?.targets || []} />
          </CardContent>
        </Card>

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
      </div>

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
