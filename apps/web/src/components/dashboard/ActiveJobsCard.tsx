import { JobProgress } from '@/components/JobProgress'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/ui/status-badge'
import { TableSkeleton } from '@/components/ui/skeleton'
import { useJobs } from '@/hooks/useJobs'
import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'

interface ActiveJobsCardProps {
  /** `active_jobs` from the dashboard — running *and* queued. */
  activeJobs?: number
}

const MAX_ROWS = 5

/**
 * What is happening right now.
 *
 * The dashboard payload counts active jobs but carries no progress, so this is
 * the one panel that reads `/api/jobs`. Only running jobs have progress to
 * show; anything queued behind them is stated as a count, because a queued job
 * has nothing to display but its name.
 */
export function ActiveJobsCard({ activeJobs }: ActiveJobsCardProps) {
  const { data, isPending } = useJobs({ status: 'running', limit: MAX_ROWS })

  const running = data?.jobs ?? []
  const queued = Math.max((activeJobs ?? running.length) - running.length, 0)

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle>In flight</CardTitle>
        <CardDescription>
          {queued > 0
            ? `${running.length} running, ${queued} queued behind ${queued === 1 ? 'it' : 'them'}.`
            : 'Live progress for every job the daemon is working on.'}
        </CardDescription>
      </CardHeader>

      <CardContent>
        {isPending ? (
          <TableSkeleton rows={2} columns={3} />
        ) : running.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {queued > 0
              ? `Nothing running yet. ${queued} ${queued === 1 ? 'job is' : 'jobs are'} queued.`
              : 'Nothing is running. The daemon is idle.'}
          </p>
        ) : (
          <ul className="space-y-4">
            {running.map((job) => (
              <li key={job.id} className="space-y-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <StatusBadge kind="job" status="running" />
                    <span className="truncate font-medium">{job.target}</span>
                    <span className="text-xs text-muted-foreground">{job.type}</span>
                  </div>
                  <Link
                    to="/jobs/$id"
                    params={{ id: job.id }}
                    className="inline-flex items-center gap-0.5 rounded-xs text-sm text-primary underline-offset-4 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    Details
                    <ChevronRight aria-hidden="true" className="h-3.5 w-3.5" />
                  </Link>
                </div>
                {job.progress ? (
                  <JobProgress progress={job.progress} />
                ) : (
                  <p className="text-xs text-muted-foreground">
                    Starting — no progress reported yet.
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
