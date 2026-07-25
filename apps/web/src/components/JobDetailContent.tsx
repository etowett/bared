import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge } from '@/components/ui/status-badge'
import { cn, formatDate } from '@/lib/utils'
import { AlertCircle } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useJob, useJobLogs } from '../hooks/useJobs'
import { useWebSocket } from '../hooks/useWebSocket'
import type { Job } from '../types'
import { JobProgress } from './JobProgress'

interface JobDetailContentProps {
  job: Job
  compact?: boolean
}

export function JobDetailContent({ job, compact: _compact = false }: JobDetailContentProps) {
  const logsEndRef = useRef<HTMLDivElement>(null)

  const { data: updatedJob } = useJob(job.id)
  const currentJob = updatedJob || job

  const { data: logsData } = useJobLogs(job.id)

  const { messages: wsMessages, connected } = useWebSocket(job.id, {
    enabled: currentJob.status === 'running' || currentJob.status === 'queued',
  })

  const allLogs = [...(logsData?.logs || []), ...wsMessages]

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [allLogs.length])

  const getLogLevelClass = (level: string): string => {
    switch (level.toLowerCase()) {
      case 'error':
        return 'text-red-400'
      case 'warn':
      case 'warning':
        return 'text-yellow-400'
      case 'info':
        return 'text-blue-400'
      case 'debug':
        return 'text-indigo-400'
      default:
        return ''
    }
  }

  return (
    <ScrollArea className="h-full">
      <div className="space-y-6 pr-4">
        {/* Job Info Grid */}
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-1">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Job ID
            </p>
            <p className="text-sm font-mono">{currentJob.id}</p>
          </div>
          <div className="space-y-1">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Type
            </p>
            <p className="text-sm">{currentJob.type}</p>
          </div>
          <div className="space-y-1">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Target
            </p>
            <p className="text-sm">{currentJob.target}</p>
          </div>
          <div className="space-y-1">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Status
            </p>
            <div className="flex items-center gap-2">
              <StatusBadge
                status={
                  currentJob.status as
                    | 'running'
                    | 'idle'
                    | 'queued'
                    | 'completed'
                    | 'failed'
                    | 'cancelled'
                    | 'cancelling'
                }
              />
              {currentJob.manual && (
                <Badge className="bg-yellow-100 text-yellow-800 text-xs">Manual</Badge>
              )}
            </div>
          </div>
          <div className="space-y-1">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Created
            </p>
            <p className="text-sm">{formatDate(currentJob.created_at)}</p>
          </div>
          {currentJob.started_at && (
            <div className="space-y-1">
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Started
              </p>
              <p className="text-sm">{formatDate(currentJob.started_at)}</p>
            </div>
          )}
          {currentJob.completed_at && (
            <div className="space-y-1">
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Completed
              </p>
              <p className="text-sm">{formatDate(currentJob.completed_at)}</p>
            </div>
          )}
          {currentJob.duration_seconds && (
            <div className="space-y-1">
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Duration
              </p>
              <p className="text-sm">{currentJob.duration_seconds.toFixed(2)}s</p>
            </div>
          )}
          {currentJob.backup_path && (
            <div className="col-span-2 space-y-1">
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Backup Path
              </p>
              <p className="text-sm break-all">{currentJob.backup_path}</p>
            </div>
          )}
        </div>

        {/* Progress Section */}
        {currentJob.progress && (
          <div className="space-y-3">
            <h3 className="text-lg font-semibold">Progress</h3>
            <JobProgress progress={currentJob.progress} />
          </div>
        )}

        {/* Error Section */}
        {currentJob.error && (
          <div className="space-y-3">
            <h3 className="text-lg font-semibold">Error</h3>
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>
                <pre className="whitespace-pre-wrap break-all font-mono text-xs">
                  {currentJob.error}
                </pre>
              </AlertDescription>
            </Alert>
          </div>
        )}

        {/* Logs Section */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold">Logs</h3>
            {connected && <span className="text-xs font-semibold text-green-600">● Live</span>}
            {!connected && (currentJob.status === 'running' || currentJob.status === 'queued') && (
              <span className="text-xs font-semibold text-yellow-600">● Connecting...</span>
            )}
          </div>

          <div className="bg-slate-800 text-slate-200 rounded p-4 max-h-96 overflow-y-auto font-mono text-xs leading-relaxed">
            {allLogs.length === 0 ? (
              <div className="text-center py-8 text-slate-500">No logs available</div>
            ) : (
              allLogs.map((log, index) => (
                <div
                  key={index}
                  className="flex gap-2 py-1 border-b border-slate-700 last:border-0"
                >
                  <span className="text-slate-500 shrink-0">{log.timestamp}</span>
                  <span className={cn('shrink-0 font-semibold', getLogLevelClass(log.level))}>
                    [{log.level.toUpperCase()}]
                  </span>
                  <span className="flex-1 break-words">{log.message}</span>
                </div>
              ))
            )}
            <div ref={logsEndRef} />
          </div>
        </div>
      </div>
    </ScrollArea>
  )
}
