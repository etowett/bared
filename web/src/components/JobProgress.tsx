import { Progress } from '@/components/ui/progress'
import { formatBytes } from '@/lib/utils'
import type { Progress as ProgressType } from '../types'

interface JobProgressProps {
  progress: ProgressType
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

  if (compact) {
    return (
      <div className="flex items-center gap-2 min-w-[120px]">
        <Progress value={Math.min(progress.percent, 100)} className="flex-1" />
        <span className="text-xs font-semibold text-muted-foreground min-w-[45px]">
          {progress.percent.toFixed(1)}%
        </span>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex justify-between text-sm">
        <span className="font-medium text-foreground">{progress.stage}</span>
        <span className="font-semibold text-primary">{progress.percent.toFixed(1)}%</span>
      </div>

      <Progress value={Math.min(progress.percent, 100)} />

      <div className="flex justify-between text-xs text-muted-foreground">
        {progress.bytes_total > 0 && (
          <div>
            {formatBytes(progress.bytes_processed)} / {formatBytes(progress.bytes_total)}
          </div>
        )}
        {progress.eta && <div>ETA: {formatETA(progress.eta)}</div>}
      </div>

      {progress.message && (
        <div className="text-xs text-muted-foreground italic">{progress.message}</div>
      )}
    </div>
  )
}
