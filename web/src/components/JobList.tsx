import { useCancelJob } from '../hooks/useJobs'
import { JobProgress } from './JobProgress'
import type { Job } from '../types'

interface JobListProps {
  jobs: Job[]
  onSelectJob: (job: Job) => void
  selectedJobId?: string
}

export function JobList({ jobs, onSelectJob, selectedJobId }: JobListProps) {
  const cancelJob = useCancelJob()

  const handleCancel = async (e: React.MouseEvent, jobId: string) => {
    e.stopPropagation()
    if (confirm('Are you sure you want to cancel this job?')) {
      try {
        await cancelJob.mutateAsync(jobId)
      } catch (error) {
        alert(`Failed to cancel job: ${error}`)
      }
    }
  }

  const formatDate = (dateStr?: string): string => {
    if (!dateStr) return 'N/A'
    try {
      return new Date(dateStr).toLocaleString()
    } catch {
      return dateStr
    }
  }

  const formatDuration = (seconds?: number): string => {
    if (!seconds) return 'N/A'
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = Math.floor(seconds % 60)

    if (hours > 0) {
      return `${hours}h ${minutes}m ${secs}s`
    } else if (minutes > 0) {
      return `${minutes}m ${secs}s`
    } else {
      return `${secs}s`
    }
  }

  const getStatusClass = (status: string): string => {
    switch (status) {
      case 'queued':
        return 'status-queued'
      case 'running':
        return 'status-running'
      case 'completed':
        return 'status-completed'
      case 'failed':
        return 'status-failed'
      case 'cancelled':
      case 'cancelling':
        return 'status-cancelled'
      default:
        return ''
    }
  }

  if (jobs.length === 0) {
    return <div className="empty-state">No jobs found</div>
  }

  return (
    <div className="job-list">
      <table className="job-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Type</th>
            <th>Target</th>
            <th>Status</th>
            <th>Progress</th>
            <th>Created</th>
            <th>Duration</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {jobs.map((job) => (
            <tr
              key={job.id}
              onClick={() => onSelectJob(job)}
              className={`job-row ${selectedJobId === job.id ? 'selected' : ''}`}
            >
              <td className="job-id">{job.id.slice(0, 8)}</td>
              <td className="job-type">{job.type}</td>
              <td className="job-target">{job.target}</td>
              <td>
                <span className={`status-badge ${getStatusClass(job.status)}`}>
                  {job.status}
                </span>
                {job.manual && <span className="manual-badge">Manual</span>}
              </td>
              <td className="job-progress">
                {job.progress ? (
                  <JobProgress progress={job.progress} compact />
                ) : (
                  <span className="progress-none">-</span>
                )}
              </td>
              <td className="job-created">{formatDate(job.created_at)}</td>
              <td className="job-duration">{formatDuration(job.duration_seconds)}</td>
              <td className="job-actions">
                {(job.status === 'running' || job.status === 'queued') && (
                  <button
                    onClick={(e) => handleCancel(e, job.id)}
                    disabled={cancelJob.isPending || job.status === 'cancelling'}
                    className="btn-danger btn-sm"
                  >
                    Cancel
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
