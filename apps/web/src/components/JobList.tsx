import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { StatusBadge } from '@/components/ui/status-badge'
import {
  SortableTableHead,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  type SortDirection,
  type TableDensity,
} from '@/components/ui/table'
import { useConfirm } from '@/contexts/ConfirmContext'
import type { JobSortKey } from '@/lib/job-search'
import { cn, formatDate, formatDuration } from '@/lib/utils'
import { toast } from 'sonner'
import { Link, useNavigate } from '@tanstack/react-router'
import { Copy, MoreHorizontal } from 'lucide-react'
import { useMemo } from 'react'
import { useCancelJob } from '../hooks/useJobs'
import type { Job } from '../types'
import { JobProgress } from './JobProgress'

const primaryCellClass =
  'rounded-sm font-mono underline-offset-4 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background'

const cancellableStatuses: Job['status'][] = ['running', 'queued', 'cancelling']

function sortValue(job: Job, key: JobSortKey): string | number {
  switch (key) {
    case 'target':
      return job.target.toLowerCase()
    case 'status':
      return job.status
    case 'duration':
      return job.duration_seconds ?? -1
    case 'created_at':
      return Date.parse(job.created_at) || 0
  }
}

interface JobListProps {
  jobs: Job[]
  onSelectJob?: (_job: Job) => void
  selectedJobId?: string
  navigationMode?: boolean
  density?: TableDensity
  /** Sort applied to `jobs` before rendering. Omit to keep the server order. */
  sort?: JobSortKey
  order?: SortDirection
  onSortChange?: (_sort: JobSortKey, _order: SortDirection) => void
}

export function JobList({
  jobs,
  onSelectJob,
  selectedJobId,
  navigationMode = false,
  density = 'comfortable',
  sort,
  order = 'desc',
  onSortChange,
}: JobListProps) {
  const navigate = useNavigate()
  const cancelJob = useCancelJob()
  const confirm = useConfirm()

  // Sorting is opt-in: without `onSortChange` the headers are plain and the
  // rows stay in the order the daemon returned them.
  const sortable = !!onSortChange

  const rows = useMemo(() => {
    if (!sort) return jobs
    const direction = order === 'asc' ? 1 : -1
    return [...jobs].sort((a, b) => {
      const left = sortValue(a, sort)
      const right = sortValue(b, sort)
      if (left < right) return -1 * direction
      if (left > right) return 1 * direction
      return 0
    })
  }, [jobs, sort, order])

  const handleRowClick = (job: Job) => {
    if (navigationMode) {
      navigate({ to: '/jobs/$id', params: { id: job.id } })
    } else if (onSelectJob) {
      onSelectJob(job)
    }
  }

  const handleCopyId = async (jobId: string) => {
    try {
      // `navigator.clipboard` is absent on insecure origins, which a daemon on
      // a LAN address very often is — so this has to fail into a toast rather
      // than an unhandled TypeError.
      await window.navigator.clipboard.writeText(jobId)
      toast.success('Job ID copied')
    } catch {
      toast.error('Could not copy the job ID', {
        description: 'The browser refused clipboard access on this page.',
      })
    }
  }

  const handleCancel = async (jobId: string) => {
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

  const sortedAs = (key: JobSortKey): SortDirection | false => (sort === key ? order : false)

  const heading = (key: JobSortKey, label: string, props?: { priority?: 'sm' | 'md' | 'lg' }) =>
    sortable ? (
      <SortableTableHead
        sorted={sortedAs(key)}
        onSort={(direction) => onSortChange?.(key, direction)}
        priority={props?.priority}
      >
        {label}
      </SortableTableHead>
    ) : (
      <TableHead priority={props?.priority}>{label}</TableHead>
    )

  if (jobs.length === 0) {
    return <div className="text-center py-12 text-muted-foreground">No jobs found</div>
  }

  return (
    <Table density={density}>
      <TableHeader>
        <TableRow>
          <TableHead className="w-24 sm:w-auto">ID</TableHead>
          <TableHead priority="sm">Type</TableHead>
          {heading('target', 'Target')}
          {heading('status', 'Status')}
          <TableHead priority="lg">Progress</TableHead>
          {heading('created_at', 'Created', { priority: 'sm' })}
          {heading('duration', 'Duration', { priority: 'md' })}
          <TableHead className="text-right">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((job) => {
          const cancellable = cancellableStatuses.includes(job.status)

          return (
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
              {/*
                Held to a fixed width at the narrowest sizes so the metadata
                folded in below the id cannot push Status and the actions menu
                off the screen. A `max-width` would not do it — an auto-layout
                table ignores that on a cell.
              */}
              <TableCell className="w-24 font-mono text-sm sm:w-auto">
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
                {/*
                  `Type` and `Created` are hidden below `sm`, and a hidden
                  column must not be the only place a value appears — so they
                  fold in here, as one string rather than two so a text query
                  still finds exactly one "backup" cell.
                */}
                <div className="mt-1 wrap-break-word text-xs font-normal text-muted-foreground sm:hidden">
                  {job.type} · {formatDate(job.created_at)}
                </div>
              </TableCell>
              <TableCell priority="sm">{job.type}</TableCell>
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
              <TableCell priority="lg">
                {job.progress ? (
                  <JobProgress progress={job.progress} compact />
                ) : (
                  <span className="text-muted-foreground">-</span>
                )}
              </TableCell>
              <TableCell priority="sm" className="font-mono text-sm">
                {formatDate(job.created_at)}
              </TableCell>
              <TableCell priority="md" className="font-mono text-sm">
                {formatDuration(job.duration_seconds)}
              </TableCell>
              {/*
                The menu lives behind one neutral trigger so a destructive
                action is not the loudest thing in every row. The cell swallows
                the click so opening the menu does not also open the job.
              */}
              <TableCell className="text-right" onClick={(event) => event.stopPropagation()}>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-8 p-0"
                      aria-label={`Actions for job ${job.id}`}
                    >
                      <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuLabel>Job {job.id.slice(0, 8)}</DropdownMenuLabel>
                    <DropdownMenuItem onSelect={() => handleRowClick(job)}>
                      View details
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => handleCopyId(job.id)}>
                      <Copy aria-hidden="true" />
                      Copy job ID
                    </DropdownMenuItem>
                    {cancellable && (
                      <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          destructive
                          disabled={cancelJob.isPending || job.status === 'cancelling'}
                          onSelect={() => handleCancel(job.id)}
                        >
                          Cancel job
                        </DropdownMenuItem>
                      </>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
