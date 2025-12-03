import { useState } from 'react'
import { clearAuth } from '../api/client'
import { useDashboard } from '../hooks/useDashboard'
import { useJobs } from '../hooks/useJobs'
import { TargetList } from './TargetList'
import { JobList } from './JobList'
import { JobDetail } from './JobDetail'
import type { Job } from '../types'

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

  const formatBytes = (bytes?: number): string => {
    if (!bytes) return 'N/A'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let size = bytes
    let unitIndex = 0
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024
      unitIndex++
    }
    return `${size.toFixed(2)} ${units[unitIndex]}`
  }

  if (dashboardLoading) {
    return (
      <div className="dashboard">
        <div className="loading">Loading dashboard...</div>
      </div>
    )
  }

  return (
    <div className="dashboard">
      <header className="dashboard-header">
        <h1>BareD - Backup Dashboard</h1>
        <button onClick={handleLogout} className="btn-secondary">
          Logout
        </button>
      </header>

      <div className="dashboard-stats">
        <div className="stat-card">
          <h3>Targets</h3>
          <div className="stat-value">{dashboard?.targets.length || 0}</div>
        </div>
        <div className="stat-card">
          <h3>Active Jobs</h3>
          <div className="stat-value">{dashboard?.active_jobs || 0}</div>
        </div>
        <div className="stat-card">
          <h3>Total Jobs</h3>
          <div className="stat-value">{dashboard?.total_jobs || 0}</div>
        </div>
        <div className="stat-card">
          <h3>Storage Used</h3>
          <div className="stat-value">{formatBytes(dashboard?.total_storage_bytes)}</div>
        </div>
      </div>

      <div className="dashboard-content">
        <div className="dashboard-section">
          <h2>Backup Targets</h2>
          <TargetList targets={dashboard?.targets || []} />
        </div>

        <div className="dashboard-section">
          <div className="section-header">
            <h2>Jobs</h2>
            <div className="filter-controls">
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="filter-select"
              >
                <option value="">All Status</option>
                <option value="queued">Queued</option>
                <option value="running">Running</option>
                <option value="completed">Completed</option>
                <option value="failed">Failed</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </div>
          </div>

          {jobsLoading ? (
            <div className="loading">Loading jobs...</div>
          ) : (
            <JobList
              jobs={jobsData?.jobs || []}
              onSelectJob={setSelectedJob}
              selectedJobId={selectedJob?.id}
            />
          )}
        </div>
      </div>

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
