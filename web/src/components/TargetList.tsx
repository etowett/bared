import { useTriggerBackup } from '../hooks/useJobs'
import type { Target } from '../types'

interface TargetListProps {
  targets: Target[]
}

export function TargetList({ targets }: TargetListProps) {
  const triggerBackup = useTriggerBackup()

  const handleBackup = async (targetName: string) => {
    if (confirm(`Start backup for ${targetName}?`)) {
      try {
        await triggerBackup.mutateAsync(targetName)
      } catch (error) {
        alert(`Failed to trigger backup: ${error}`)
      }
    }
  }

  const formatDate = (dateStr?: string): string => {
    if (!dateStr) return 'Never'
    try {
      return new Date(dateStr).toLocaleString()
    } catch {
      return dateStr
    }
  }

  if (targets.length === 0) {
    return <div className="empty-state">No targets configured</div>
  }

  return (
    <div className="target-list">
      {targets.map((target) => (
        <div key={target.name} className="target-card">
          <div className="target-header">
            <h3>{target.name}</h3>
            <span className={`status-badge ${target.is_running ? 'running' : 'idle'}`}>
              {target.is_running ? 'Running' : 'Idle'}
            </span>
          </div>

          <div className="target-info">
            <div className="info-row">
              <span className="info-label">Type:</span>
              <span className="info-value">{target.type}</span>
            </div>
            <div className="info-row">
              <span className="info-label">Database:</span>
              <span className="info-value">{target.database}</span>
            </div>
            <div className="info-row">
              <span className="info-label">Last Backup:</span>
              <span className="info-value">{formatDate(target.last_backup)}</span>
            </div>
            {target.schedule && (
              <div className="info-row">
                <span className="info-label">Schedule:</span>
                <span className="info-value">{target.schedule}</span>
              </div>
            )}
            {target.next_scheduled && (
              <div className="info-row">
                <span className="info-label">Next Run:</span>
                <span className="info-value">{formatDate(target.next_scheduled)}</span>
              </div>
            )}
          </div>

          <button
            onClick={() => handleBackup(target.name)}
            disabled={target.is_running || triggerBackup.isPending}
            className="btn-primary"
          >
            {target.is_running ? 'Backup Running...' : 'Backup Now'}
          </button>
        </div>
      ))}
    </div>
  )
}
