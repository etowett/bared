import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useConfirm } from '@/contexts/ConfirmContext'
import { useTriggerBackup } from '@/hooks/useJobs'
import type { Target } from '@/types'
import { describeSchedule, formatNextRun, serverZoneLabel } from '@/utils/cron'
import { toast } from 'sonner'

interface NextRunsCardProps {
  targets: Target[]
}

/** Only the next five: past that it is a calendar, not a dashboard. */
const MAX_ROWS = 5

/**
 * What the daemon is about to do, soonest first — and the one place to make it
 * happen sooner.
 */
export function NextRunsCard({ targets }: NextRunsCardProps) {
  const triggerBackup = useTriggerBackup()
  const confirm = useConfirm()

  const scheduled = targets
    .filter((target) => target.schedule && target.next_scheduled)
    .sort((a, b) => Date.parse(a.next_scheduled!) - Date.parse(b.next_scheduled!))

  // Every `next_scheduled` carries the daemon's own UTC offset, so any one of
  // them identifies the zone its cron expressions are read in.
  const zone = serverZoneLabel(scheduled[0]?.next_scheduled)

  const handleRunNow = async (targetName: string) => {
    const confirmed = await confirm({
      title: 'Start backup',
      description: `Start a backup of "${targetName}" now? Its schedule is unaffected.`,
      confirmLabel: 'Start backup',
      variant: 'default',
    })
    if (!confirmed) return

    try {
      await triggerBackup.mutateAsync(targetName)
      toast.success(`Backup of "${targetName}" queued`)
    } catch (error) {
      toast.error(`Could not start the backup of "${targetName}"`, {
        description: error instanceof Error ? error.message : String(error),
      })
    }
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle>Next scheduled runs</CardTitle>
        <CardDescription>
          Shown in your timezone
          {zone ? `; the daemon schedules in ${zone}.` : '.'}
        </CardDescription>
      </CardHeader>

      <CardContent>
        {scheduled.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            No target has a schedule. Backups run only when you start them.
          </p>
        ) : (
          <ul className="divide-y">
            {scheduled.slice(0, MAX_ROWS).map((target) => (
              <li
                key={target.name}
                className="flex flex-wrap items-center justify-between gap-2 py-3 first:pt-0 last:pb-0"
              >
                <div className="min-w-0">
                  <p className="font-medium">{target.name}</p>
                  <p className="text-sm text-muted-foreground">
                    {formatNextRun(target.next_scheduled)}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {describeSchedule(target.schedule!, target.next_scheduled)}
                  </p>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={target.is_running || triggerBackup.isPending}
                  onClick={() => handleRunNow(target.name)}
                >
                  {target.is_running ? 'Running' : 'Run now'}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
