import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/ui/status-badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { describeTargetHealth } from '@/lib/status'
import type { Target } from '@/types'
import { formatAge } from '@/utils/age'
import { formatNextRun } from '@/utils/cron'
import { Link } from '@tanstack/react-router'
import { rankTargets } from './health'
import { formatRuntime, formatSize } from './format'
import { UnknownValue } from './UnknownValue'

interface TargetHealthTableProps {
  targets: Target[]
}

/**
 * Every target, ordered by how much it should worry someone.
 *
 * Reading order is triage order — failing, then overdue, then never run, then
 * in flight, then unreported, then healthy — so the table needs no sort control
 * to answer "what do I look at first". Each row carries an `id` the attention
 * banner links to.
 */
export function TargetHealthTable({ targets }: TargetHealthTableProps) {
  const ranked = rankTargets(targets)

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle>Target health</CardTitle>
        <CardDescription>Most urgent first. Ages are relative to now.</CardDescription>
      </CardHeader>

      <CardContent>
        {ranked.length === 0 ? (
          <div className="py-10 text-center">
            <p className="text-sm font-medium">No targets configured</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Add a target and the daemon starts backing it up on schedule.
            </p>
            <Link
              to="/config/targets"
              className="mt-4 inline-flex rounded-xs text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            >
              Configure targets
            </Link>
          </div>
        ) : (
          <div className="overflow-x-auto">
            {/* Same floor as JobList and TargetList: scroll sideways rather
                than crush the cells. */}
            <Table className="min-w-[42rem] [&_td]:px-3 [&_th]:px-3">
              <TableHeader>
                <TableRow>
                  <TableHead>Target</TableHead>
                  <TableHead>Health</TableHead>
                  <TableHead>Last success</TableHead>
                  <TableHead>Next run</TableHead>
                  <TableHead>Last size</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ranked.map((target) => (
                  <TableRow key={target.name} id={`target-${target.name}`} className="scroll-mt-24">
                    <TableCell className="align-top">
                      <div className="font-medium">{target.name}</div>
                      <div className="font-mono text-xs text-muted-foreground">
                        {target.type} · {target.database}
                      </div>
                    </TableCell>

                    {/* The failure streak is the evidence behind the verdict,
                        not a fact of its own, so it sits under the badge
                        rather than in a column of its own. */}
                    <TableCell className="align-top">
                      <div className="flex flex-col items-start gap-1">
                        <StatusBadge kind="target" status={describeTargetHealth(target)} />
                        {(target.consecutive_failures ?? 0) > 0 && (
                          <span className="text-xs text-muted-foreground">
                            {target.consecutive_failures} failed in a row
                          </span>
                        )}
                      </div>
                    </TableCell>

                    <TableCell className="align-top text-sm">
                      <LastSuccess target={target} />
                    </TableCell>

                    <TableCell className="align-top text-sm">
                      <NextRun target={target} />
                    </TableCell>

                    <TableCell className="align-top font-mono text-sm tabular-nums">
                      <LastSize target={target} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

/**
 * "never" and "we were not told" look identical on the wire — both arrive as an
 * absent `last_backup` — so the status field is what separates them.
 */
function LastSuccess({ target }: { target: Target }) {
  const age = formatAge(target.last_backup)
  if (age) return <span>{age}</span>
  if (target.last_backup_status === 'never')
    return <span className="text-muted-foreground">never</span>
  return <UnknownValue title="This daemon did not report a last backup time." />
}

function NextRun({ target }: { target: Target }) {
  if (!target.schedule) return <span className="text-muted-foreground">manual only</span>
  if (!target.next_scheduled) {
    return <UnknownValue title="The daemon has not scheduled the next run yet." />
  }
  // `formatNextRun` renders the instant in the viewer's own zone, which is the
  // only timezone-honest way to state a cron schedule to a browser. It returns
  // "in 3 hours · Today at 5:56 AM". The relative half leads and the wall-clock
  // half drops to a second line, so this column is not twice as wide as
  // anything it sits next to.
  const [relative, absolute] = formatNextRun(target.next_scheduled).split(' · ')

  return (
    <div className="whitespace-nowrap">
      <div>{relative}</div>
      {absolute && <div className="text-xs text-muted-foreground">{absolute}</div>}
    </div>
  )
}

function LastSize({ target }: { target: Target }) {
  if (target.last_backup_bytes === undefined) {
    return <UnknownValue title="The size of the last backup was not recorded." />
  }
  return (
    <div className="whitespace-nowrap">
      <div>{formatSize(target.last_backup_bytes)}</div>
      {target.last_backup_duration_seconds !== undefined && (
        <div className="text-xs text-muted-foreground">
          in {formatRuntime(target.last_backup_duration_seconds)}
        </div>
      )}
    </div>
  )
}
