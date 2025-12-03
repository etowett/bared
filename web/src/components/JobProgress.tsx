import type { Progress } from '../types'

interface JobProgressProps {
  progress: Progress
  compact?: boolean
}

export function JobProgress({ progress, compact = false }: JobProgressProps) {
  const formatETA = (eta?: string): string => {
    if (!eta) return 'Calculating...'
    try {
      const etaDate = new Date(eta)
      const now = new Date()
      const diffMs = etaDate.getTime() - now.getTime()

      if (diffMs < 0) return 'Soon'

      const diffSeconds = Math.floor(diffMs / 1000)
      const hours = Math.floor(diffSeconds / 3600)
      const minutes = Math.floor((diffSeconds % 3600) / 60)
      const seconds = diffSeconds % 60

      if (hours > 0) {
        return `${hours}h ${minutes}m`
      } else if (minutes > 0) {
        return `${minutes}m ${seconds}s`
      } else {
        return `${seconds}s`
      }
    } catch {
      return eta
    }
  }

  const formatBytes = (bytes: number): string => {
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let size = bytes
    let unitIndex = 0
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024
      unitIndex++
    }
    return `${size.toFixed(2)} ${units[unitIndex]}`
  }

  if (compact) {
    return (
      <div className="job-progress-compact">
        <div className="progress-bar">
          <div className="progress-fill" style={{ width: `${Math.min(progress.percent, 100)}%` }} />
        </div>
        <span className="progress-text">{progress.percent.toFixed(1)}%</span>
      </div>
    )
  }

  return (
    <div className="job-progress-full">
      <div className="progress-header">
        <span className="progress-stage">{progress.stage}</span>
        <span className="progress-percent">{progress.percent.toFixed(1)}%</span>
      </div>

      <div className="progress-bar">
        <div className="progress-fill" style={{ width: `${Math.min(progress.percent, 100)}%` }} />
      </div>

      <div className="progress-details">
        {progress.bytes_total > 0 && (
          <div className="progress-bytes">
            {formatBytes(progress.bytes_processed)} / {formatBytes(progress.bytes_total)}
          </div>
        )}
        {progress.eta && <div className="progress-eta">ETA: {formatETA(progress.eta)}</div>}
      </div>

      {progress.message && <div className="progress-message">{progress.message}</div>}
    </div>
  )
}
