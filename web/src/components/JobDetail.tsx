import { useEffect, useRef } from 'react'
import { useJob, useJobLogs } from '../hooks/useJobs'
import { useWebSocket } from '../hooks/useWebSocket'
import { JobProgress } from './JobProgress'
import type { Job } from '../types'

interface JobDetailProps {
  job: Job
  onClose: () => void
}

export function JobDetail({ job, onClose }: JobDetailProps) {
  const logsEndRef = useRef<HTMLDivElement>(null)

  // Fetch updated job details
  const { data: updatedJob } = useJob(job.id)
  const currentJob = updatedJob || job

  // Fetch historical logs
  const { data: logsData } = useJobLogs(job.id)

  // WebSocket for real-time logs (only for running jobs)
  const { messages: wsMessages, connected } = useWebSocket(job.id, {
    enabled: currentJob.status === 'running' || currentJob.status === 'queued',
  })

  // Combine historical and real-time logs
  const allLogs = [...(logsData?.logs || []), ...wsMessages]

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [allLogs.length])

  const formatDate = (dateStr?: string): string => {
    if (!dateStr) return 'N/A'
    try {
      return new Date(dateStr).toLocaleString()
    } catch {
      return dateStr
    }
  }

  const getLogLevelClass = (level: string): string => {
    switch (level.toLowerCase()) {
      case 'error':
        return 'log-error'
      case 'warn':
      case 'warning':
        return 'log-warning'
      case 'info':
        return 'log-info'
      case 'debug':
        return 'log-debug'
      default:
        return ''
    }
  }

  return (
    <div className="job-detail-overlay" onClick={onClose}>
      <div className="job-detail-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Job Details</h2>
          <button onClick={onClose} className="btn-close">
            ×
          </button>
        </div>

        <div className="modal-body">
          <div className="job-info-grid">
            <div className="info-item">
              <span className="info-label">Job ID:</span>
              <span className="info-value">{currentJob.id}</span>
            </div>
            <div className="info-item">
              <span className="info-label">Type:</span>
              <span className="info-value">{currentJob.type}</span>
            </div>
            <div className="info-item">
              <span className="info-label">Target:</span>
              <span className="info-value">{currentJob.target}</span>
            </div>
            <div className="info-item">
              <span className="info-label">Status:</span>
              <span className="info-value">
                <span className={`status-badge status-${currentJob.status}`}>
                  {currentJob.status}
                </span>
                {currentJob.manual && <span className="manual-badge">Manual</span>}
              </span>
            </div>
            <div className="info-item">
              <span className="info-label">Created:</span>
              <span className="info-value">{formatDate(currentJob.created_at)}</span>
            </div>
            {currentJob.started_at && (
              <div className="info-item">
                <span className="info-label">Started:</span>
                <span className="info-value">{formatDate(currentJob.started_at)}</span>
              </div>
            )}
            {currentJob.completed_at && (
              <div className="info-item">
                <span className="info-label">Completed:</span>
                <span className="info-value">{formatDate(currentJob.completed_at)}</span>
              </div>
            )}
            {currentJob.duration_seconds && (
              <div className="info-item">
                <span className="info-label">Duration:</span>
                <span className="info-value">{currentJob.duration_seconds.toFixed(2)}s</span>
              </div>
            )}
            {currentJob.backup_path && (
              <div className="info-item info-item-full">
                <span className="info-label">Backup Path:</span>
                <span className="info-value">{currentJob.backup_path}</span>
              </div>
            )}
          </div>

          {currentJob.progress && (
            <div className="progress-section">
              <h3>Progress</h3>
              <JobProgress progress={currentJob.progress} />
            </div>
          )}

          {currentJob.error && (
            <div className="error-section">
              <h3>Error</h3>
              <pre className="error-message">{currentJob.error}</pre>
            </div>
          )}

          <div className="logs-section">
            <div className="logs-header">
              <h3>Logs</h3>
              {connected && (
                <span className="ws-status ws-connected">● Live</span>
              )}
              {!connected && (currentJob.status === 'running' || currentJob.status === 'queued') && (
                <span className="ws-status ws-disconnected">● Connecting...</span>
              )}
            </div>

            <div className="logs-viewer">
              {allLogs.length === 0 ? (
                <div className="logs-empty">No logs available</div>
              ) : (
                allLogs.map((log, index) => (
                  <div key={index} className={`log-entry ${getLogLevelClass(log.level)}`}>
                    <span className="log-timestamp">{log.timestamp}</span>
                    <span className="log-level">[{log.level.toUpperCase()}]</span>
                    <span className="log-message">{log.message}</span>
                  </div>
                ))
              )}
              <div ref={logsEndRef} />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
