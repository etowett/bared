import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/ui/status-badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useConfirm } from '@/contexts/ConfirmContext'
import { cn, formatDate, formatDuration } from '@/lib/utils'
import { toast } from 'sonner'
import { useNavigate } from '@tanstack/react-router'
import { useCancelJob } from '../hooks/useJobs'
import type { Job } from '../types'
import { JobProgress } from './JobProgress'

interface JobListProps {
  jobs: Job[]
  onSelectJob?: (_job: Job) => void
  selectedJobId?: string
  navigationMode?: boolean
}

export function JobList({
  jobs,
  onSelectJob,
  selectedJobId,
  navigationMode = false,
}: JobListProps) {
  const navigate = useNavigate()
  const cancelJob = useCancelJob()
  const confirm = useConfirm()

  const handleRowClick = (job: Job) => {
    if (navigationMode) {
      navigate({ to: '/jobs/$id', params: { id: job.id } })
    } else if (onSelectJob) {
      onSelectJob(job)
    }
  }

  const handleCancel = async (e: React.MouseEvent, jobId: string) => {
    e.stopPropagation()
    const confirmed = await confirm({
      title: 'Cancel Job',
      description: 'Are you sure you want to cancel this job?',
      confirmLabel: 'Cancel Job',
      cancelLabel: 'Keep Running',
      variant: 'destructive',
    })

    if (confirmed) {
      try {
        await cancelJob.mutateAsync(jobId)
        toast.success('Job cancellation requested')
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to cancel job'
        toast.error('Failed to cancel job', {
          description: errorMessage,
        })
      }
    }
  }

  if (jobs.length === 0) {
    return <div className="text-center py-12 text-muted-foreground">No jobs found</div>
  }

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Target</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Progress</TableHead>
            <TableHead>Created</TableHead>
            <TableHead>Duration</TableHead>
            <TableHead>Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.map((job) => (
            <TableRow
              key={job.id}
              onClick={() => handleRowClick(job)}
              className={cn(
                'cursor-pointer transition-colors',
                'hover:bg-accent/50',
                selectedJobId === job.id && 'bg-primary/10 hover:bg-primary/15'
              )}
            >
              <TableCell className="font-mono text-sm">{job.id.slice(0, 8)}</TableCell>
              <TableCell>{job.type}</TableCell>
              <TableCell>{job.target}</TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <StatusBadge
                    status={
                      job.status as
                        | 'running'
                        | 'idle'
                        | 'queued'
                        | 'completed'
                        | 'failed'
                        | 'cancelled'
                        | 'cancelling'
                    }
                  />
                  {job.triggered_by === 'schedule' ? (
                    <Badge className="bg-blue-500/20 text-blue-400 border border-blue-500/30 text-xs flex items-center gap-1">
                      <svg
                        className="w-3 h-3"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
                        />
                      </svg>
                      Scheduled
                    </Badge>
                  ) : job.manual ? (
                    <Badge className="bg-terminal-warning/20 text-terminal-yellow border border-terminal-warning/30 text-xs">
                      Manual
                    </Badge>
                  ) : null}
                </div>
              </TableCell>
              <TableCell>
                {job.progress ? (
                  <JobProgress progress={job.progress} compact />
                ) : (
                  <span className="text-muted-foreground">-</span>
                )}
              </TableCell>
              <TableCell className="font-mono text-sm">{formatDate(job.created_at)}</TableCell>
              <TableCell className="font-mono text-sm">
                {formatDuration(job.duration_seconds)}
              </TableCell>
              <TableCell>
                {(job.status === 'running' ||
                  job.status === 'queued' ||
                  job.status === 'cancelling') && (
                  <Button
                    onClick={(e) => handleCancel(e, job.id)}
                    disabled={cancelJob.isPending || job.status === 'cancelling'}
                    variant="destructive"
                    size="sm"
                  >
                    Cancel
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
