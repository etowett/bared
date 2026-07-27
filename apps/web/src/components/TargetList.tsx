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
import { formatDate } from '@/lib/utils'
import { describeSchedule, formatNextRun } from '@/utils/cron'
import { toast } from 'sonner'
import { useTriggerBackup } from '../hooks/useJobs'
import type { Target } from '../types'

interface TargetListProps {
  targets: Target[]
}

export function TargetList({ targets }: TargetListProps) {
  const triggerBackup = useTriggerBackup()
  const confirm = useConfirm()

  const handleBackup = async (e: React.MouseEvent, targetName: string) => {
    e.stopPropagation()
    const confirmed = await confirm({
      title: 'Start Backup',
      description: `Are you sure you want to start a backup for ${targetName}?`,
      confirmLabel: 'Start Backup',
      variant: 'default',
    })

    if (confirmed) {
      try {
        await triggerBackup.mutateAsync(targetName)
        toast.success('Backup started successfully')
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to trigger backup'
        toast.error('Failed to trigger backup', {
          description: errorMessage,
        })
      }
    }
  }

  if (targets.length === 0) {
    return <div className="text-center py-12 text-muted-foreground">No targets found</div>
  }

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Database</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Schedule</TableHead>
            <TableHead>Last Backup</TableHead>
            <TableHead>Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {targets.map((target) => (
            <TableRow key={target.name}>
              <TableCell className="font-medium">{target.name}</TableCell>
              <TableCell>
                <StatusBadge kind="database" status={target.type} />
              </TableCell>
              <TableCell className="font-mono text-sm">{target.database}</TableCell>
              <TableCell>
                <StatusBadge status={target.is_running ? 'running' : 'idle'} />
              </TableCell>
              <TableCell className="text-sm">
                {target.schedule ? (
                  <div className="flex flex-col gap-0.5">
                    <span>{describeSchedule(target.schedule, target.next_scheduled)}</span>
                    {/* The cron line states the daemon's zone; this one is the
                        same schedule in the viewer's, which is what someone
                        actually wants to know. */}
                    {target.next_scheduled && (
                      <span className="text-xs text-muted-foreground">
                        next {formatNextRun(target.next_scheduled)}
                      </span>
                    )}
                  </div>
                ) : (
                  '—'
                )}
              </TableCell>
              <TableCell className="font-mono text-sm">
                {target.last_backup ? formatDate(target.last_backup) : 'Never'}
              </TableCell>
              <TableCell>
                <Button
                  onClick={(e) => handleBackup(e, target.name)}
                  disabled={target.is_running || triggerBackup.isPending}
                  size="sm"
                >
                  {target.is_running ? 'Running...' : 'Backup Now'}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
