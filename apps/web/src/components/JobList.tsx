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
import { Link, useNavigate } from '@tanstack/react-router'
import { useCancelJob } from '../hooks/useJobs'
import type { Job } from '../types'
import { JobProgress } from './JobProgress'

const primaryCellClass =
  'rounded-sm font-mono underline-offset-4 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background'

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
      {/*
        Eight columns do not fit a phone. Until the table gets column priority,
        give it a floor so it scrolls sideways honestly instead of crushing
        every cell into an unreadable two-character column.
      */}
      <Table className="min-w-[56rem]">
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
              {/*
                The id cell is the row's real control. In navigation mode it is
                an actual link, so middle-click, "open in new tab" and "copy
                link" all work and the row is reachable by keyboard. In dialog
                mode there is no URL to link to, so it is a button — either way
                it takes focus and shows a focus ring.
              */}
              <TableCell className="font-mono text-sm">
                {navigationMode ? (
                  <Link
                    to="/jobs/$id"
                    params={{ id: job.id }}
                    onClick={(event) => event.stopPropagation()}
                    // The full id, not the truncated one: job ids share a date
                    // prefix, so the visible 8 characters are identical across
                    // a page of rows and would give every link in the table the
                    // same accessible name.
                    aria-label={`Job ${job.id}`}
                    className={primaryCellClass}
                  >
                    {job.id.slice(0, 8)}
                  </Link>
                ) : (
                  <button
                    type="button"
                    onClick={(event) => {
                      event.stopPropagation()
                      handleRowClick(job)
                    }}
                    aria-label={`Job details for ${job.id}`}
                    className={primaryCellClass}
                  >
                    {job.id.slice(0, 8)}
                  </button>
                )}
              </TableCell>
              <TableCell>{job.type}</TableCell>
              <TableCell>{job.target}</TableCell>
              <TableCell>
                <div className="flex flex-wrap items-center gap-2">
                  <StatusBadge status={job.status} />
                  {job.triggered_by === 'schedule' ? (
                    <StatusBadge kind="trigger" status="schedule" />
                  ) : job.manual ? (
                    <StatusBadge kind="trigger" status="manual" />
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
