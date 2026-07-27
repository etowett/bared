import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge } from '@/components/ui/status-badge'
import { usePrefersReducedMotion } from '@/hooks/usePrefersReducedMotion'
import { cn, formatDate } from '@/lib/utils'
import { AlertCircle, LoaderCircle, RefreshCw, Radio, WifiOff } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useJob, useJobLogs } from '../hooks/useJobs'
import { useWebSocket } from '../hooks/useWebSocket'
import type { Job } from '../types'
import { JobProgress } from './JobProgress'

interface JobDetailContentProps {
  job: Job
  compact?: boolean
}

/** Log levels map onto the terminal palette — no literal colours at the call site. */
const logLevelClasses: Record<string, string> = {
  error: 'text-terminal-error',
  warn: 'text-terminal-warning',
  warning: 'text-terminal-warning',
  info: 'text-terminal-info',
  debug: 'text-terminal-debug',
  success: 'text-terminal-success',
}

export function JobDetailContent({ job, compact: _compact = false }: JobDetailContentProps) {
  const logsEndRef = useRef<HTMLDivElement>(null)
  const prefersReducedMotion = usePrefersReducedMotion()

  const { data: updatedJob } = useJob(job.id)
  const currentJob = updatedJob || job

  const { data: logsData } = useJobLogs(job.id)

  const isStreaming = currentJob.status === 'running' || currentJob.status === 'queued'
  const {
    messages: wsMessages,
    connected,
    error: streamError,
    reconnect,
  } = useWebSocket(job.id, { enabled: isStreaming })

  const allLogs = [...(logsData?.logs || []), ...wsMessages]

  useEffect(() => {
    // `behavior: 'smooth'` ignores prefers-reduced-motion, so honour it here.
    logsEndRef.current?.scrollIntoView({
      behavior: prefersReducedMotion ? 'auto' : 'smooth',
    })
  }, [allLogs.length, prefersReducedMotion])

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
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge status={currentJob.status} />
              {currentJob.manual && <StatusBadge kind="trigger" status="manual" />}
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
            <h3 className="text-section-title font-semibold tracking-tight">Progress</h3>
            <JobProgress progress={currentJob.progress} />
          </div>
        )}

        {/* Error Section */}
        {currentJob.error && (
          <div className="space-y-3">
            <h3 className="text-section-title font-semibold tracking-tight">Error</h3>
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
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-section-title font-semibold tracking-tight">Logs</h3>

            {/* Every stream state is a glyph plus a word — the tint only
                reinforces it, so the state survives with colour removed. */}
            {connected ? (
              <StatusBadge kind="custom" tone="success" label="Live" icon={Radio} />
            ) : isStreaming && streamError ? (
              <div className="flex items-center gap-2">
                <StatusBadge kind="custom" tone="danger" label="Disconnected" icon={WifiOff} />
                <Button variant="outline" size="sm" onClick={reconnect} className="gap-1.5">
                  <RefreshCw aria-hidden="true" className="h-3.5 w-3.5" />
                  Reconnect
                </Button>
              </div>
            ) : isStreaming ? (
              <StatusBadge
                kind="custom"
                tone="warning"
                label="Connecting"
                icon={LoaderCircle}
                className="[&>svg]:motion-safe:animate-spin"
              />
            ) : null}
          </div>

          <div className="logs-viewer max-h-96 overflow-y-auto">
            {allLogs.length === 0 ? (
              <div className="py-8 text-center text-terminal-muted">No logs available</div>
            ) : (
              allLogs.map((log, index) => (
                <div
                  key={index}
                  className="flex gap-2 border-b border-terminal-border py-1 last:border-0"
                >
                  <span className="shrink-0 text-terminal-muted">{log.timestamp}</span>
                  <span
                    className={cn(
                      'shrink-0 font-semibold',
                      logLevelClasses[log.level.toLowerCase()] ?? 'text-terminal-foreground'
                    )}
                  >
                    [{log.level.toUpperCase()}]
                  </span>
                  <span className="flex-1 wrap-break-word">{log.message}</span>
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
